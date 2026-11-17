// Command ai-eval runs a small live evaluation set against the assistant SSE
// endpoints. It is an integration tool, not part of `make verify`: it needs the
// deployed stack, demo data and (for real quality) a configured model provider.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type caseSpec struct {
	ID           string   `yaml:"id"`
	Surface      string   `yaml:"surface"`
	Prompt       string   `yaml:"prompt"`
	Contains     []string `yaml:"contains"`
	NotContains  []string `yaml:"not_contains"`
	MaxLatencyMS int      `yaml:"max_latency_ms"`
}

type suite struct {
	Cases []caseSpec `yaml:"cases"`
}

type event struct {
	Type  string `json:"type"`
	Delta string `json:"delta"`
	Error string `json:"error"`
}

type result struct {
	ID       string   `json:"id"`
	Passed   bool     `json:"passed"`
	Latency  int64    `json:"latency_ms"`
	Chars    int      `json:"chars"`
	Error    string   `json:"error,omitempty"`
	Failures []string `json:"failures,omitempty"`
	Snippet  string   `json:"snippet,omitempty"`
}

func main() {
	var (
		casesPath = flag.String("cases", "deploy/eval/ai-cases.yaml", "YAML case file")
		userURL   = flag.String("user-url", env("AI_EVAL_USER_URL", "http://127.0.0.1:18080"), "public BFF base URL")
		adminURL  = flag.String("admin-url", env("AI_EVAL_ADMIN_URL", "http://127.0.0.1:18081"), "admin BFF base URL")
		jsonOut   = flag.Bool("json", false, "print JSON report")
		delay     = flag.Duration("delay", 500*time.Millisecond, "pause between cases")
	)
	flag.Parse()

	raw, err := os.ReadFile(*casesPath)
	if err != nil {
		fatal(err)
	}
	var s suite
	if err := yaml.Unmarshal(raw, &s); err != nil {
		fatal(err)
	}
	if len(s.Cases) == 0 {
		fatal(fmt.Errorf("no cases in %s", *casesPath))
	}

	userToken, err := login(*userURL, env("AI_EVAL_USER", "demo"), env("AI_EVAL_PASSWORD", "user123"))
	if err != nil {
		fatal(fmt.Errorf("user login: %w", err))
	}
	adminToken := ""
	for _, c := range s.Cases {
		if c.Surface == "admin" {
			adminToken, err = login(*userURL, env("AI_EVAL_ADMIN", "admin"), env("AI_EVAL_ADMIN_PASSWORD", "admin123"))
			if err != nil {
				fatal(fmt.Errorf("admin login: %w", err))
			}
			break
		}
	}

	results := make([]result, 0, len(s.Cases))
	for _, c := range s.Cases {
		r := runCase(c, *userURL, *adminURL, userToken, adminToken)
		results = append(results, r)
		fmt.Printf("%-18s %-4s %6dms %5d chars", c.ID, passLabel(r.Passed), r.Latency, r.Chars)
		if r.Error != "" {
			fmt.Printf("  error=%s", r.Error)
		}
		if !r.Passed && r.Snippet != "" {
			fmt.Printf("  text=%.120q", r.Snippet)
		}
		for _, f := range r.Failures {
			fmt.Printf("  ! %s", f)
		}
		fmt.Println()
		time.Sleep(*delay)
	}

	passed := 0
	for _, r := range results {
		if r.Passed {
			passed++
		}
	}
	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{"total": len(results), "passed": passed, "results": results})
	}
	fmt.Printf("\nsummary: %d/%d passed\n", passed, len(results))
	if passed != len(results) {
		os.Exit(1)
	}
}

func runCase(c caseSpec, userURL, adminURL, userToken, adminToken string) result {
	base, token := userURL, userToken
	path := "/api/v1/ai/chat"
	if c.Surface == "admin" {
		base, token = adminURL, adminToken
		path = "/api/admin/v1/ai/chat"
	}
	start := time.Now()
	session := fmt.Sprintf("eval-%s-%d", c.ID, time.Now().UnixNano())
	text, errText, err := stream(base+path, token, c.Prompt, session)
	r := result{ID: c.ID, Latency: time.Since(start).Milliseconds(), Chars: len([]rune(text))}
	if err != nil {
		r.Error = err.Error()
		return r
	}
	if errText != "" {
		r.Error = errText
		return r
	}
	r.Snippet = text
	low := strings.ToLower(text)
	for _, want := range c.Contains {
		if !strings.Contains(low, strings.ToLower(want)) {
			r.Failures = append(r.Failures, "missing "+want)
		}
	}
	for _, bad := range c.NotContains {
		if strings.Contains(low, strings.ToLower(bad)) {
			r.Failures = append(r.Failures, "unexpected "+bad)
		}
	}
	if c.MaxLatencyMS > 0 && r.Latency > int64(c.MaxLatencyMS) {
		r.Failures = append(r.Failures, fmt.Sprintf("latency %dms > %dms", r.Latency, c.MaxLatencyMS))
	}
	r.Passed = len(r.Failures) == 0
	return r
}

func stream(url, token, prompt, session string) (string, string, error) {
	body, _ := json.Marshal(map[string]string{"message": prompt, "session_id": session})
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{Timeout: 120 * time.Second}).Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", "", fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var text, errText strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var ev event
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev); err != nil {
			continue
		}
		switch ev.Type {
		case "delta":
			text.WriteString(ev.Delta)
		case "error":
			errText.WriteString(ev.Error)
		case "done":
			return text.String(), errText.String(), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return text.String(), errText.String(), err
	}
	return text.String(), errText.String(), nil
}

func login(base, user, password string) (string, error) {
	body, _ := json.Marshal(map[string]string{"username": user, "password": password})
	resp, err := http.Post(base+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var reply struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&reply); err != nil {
		return "", err
	}
	if reply.AccessToken == "" {
		return "", fmt.Errorf("empty access token")
	}
	return reply.AccessToken, nil
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func passLabel(ok bool) string {
	if ok {
		return "PASS"
	}
	return "FAIL"
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "ai-eval:", err)
	os.Exit(2)
}
