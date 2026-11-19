// Command ai-rag-eval measures retrieval hit@k against the deployed admin RAG
// endpoint. It is a live quality harness, not part of `make verify`.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type caseSpec struct {
	ID          string   `yaml:"id"`
	Query       string   `yaml:"query"`
	ExpectedAny []string `yaml:"expected_any"`
}

type suite struct {
	Cases []caseSpec `yaml:"cases"`
}

func main() {
	casesPath := flag.String("cases", "deploy/eval/rag-cases.yaml", "YAML cases")
	adminURL := flag.String("admin-url", env("AI_RAG_EVAL_ADMIN_URL", "http://127.0.0.1:18081"), "admin BFF")
	userURL := flag.String("user-url", env("AI_RAG_EVAL_USER_URL", "http://127.0.0.1:18080"), "gateway for login")
	k := flag.Int("k", 5, "retrieval cutoff")
	flag.Parse()

	raw, err := os.ReadFile(*casesPath)
	if err != nil {
		fatal(err)
	}
	var s suite
	if err := yaml.Unmarshal(raw, &s); err != nil {
		fatal(err)
	}
	token, err := login(*userURL, env("AI_EVAL_ADMIN", "admin"), env("AI_EVAL_ADMIN_PASSWORD", "admin123"))
	if err != nil {
		fatal(err)
	}
	hits := 0
	for _, c := range s.Cases {
		docs, err := search(*adminURL, token, c.Query, *k)
		if err != nil {
			fmt.Printf("%-18s ERROR %v\n", c.ID, err)
			continue
		}
		hit := false
		var titles []string
		for _, d := range docs {
			titles = append(titles, d.Title)
			for _, want := range c.ExpectedAny {
				if strings.Contains(d.Title, want) || strings.Contains(d.Content, want) {
					hit = true
				}
			}
		}
		if hit {
			hits++
		}
		label := "MISS"
		if hit {
			label = "HIT"
		}
		fmt.Printf("%-18s %s  %v\n", c.ID, label, titles)
	}
	rate := 0.0
	if len(s.Cases) > 0 {
		rate = float64(hits) / float64(len(s.Cases))
	}
	fmt.Printf("\nhit@%d: %d/%d (%.1f%%)\n", *k, hits, len(s.Cases), rate*100)
	if hits != len(s.Cases) {
		os.Exit(1)
	}
}

type doc struct {
	Title   string  `json:"title"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

func search(base, token, query string, k int) ([]doc, error) {
	u := fmt.Sprintf("%s/api/admin/v1/ai/rag/search?q=%s&limit=%d", base, url.QueryEscape(query), k)
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	var body struct {
		Docs []doc `json:"docs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	return body.Docs, nil
}

func login(base, user, pass string) (string, error) {
	body, _ := json.Marshal(map[string]string{"username": user, "password": pass})
	resp, err := http.Post(base+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("login http %d", resp.StatusCode)
	}
	var reply struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&reply); err != nil {
		return "", err
	}
	return reply.AccessToken, nil
}

func env(k, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return fallback
}

func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(2) }
