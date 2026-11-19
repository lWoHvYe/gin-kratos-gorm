// Command ai-pref-eval is a small A/B benchmark: identical prompt with and
// without a server-side preference. It asserts the preferred brand becomes the
// first recommendation only when saved.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const prompt = "推荐一款手机"

var products = []string{"云舟 M3", "极光 Note 12", "星环 X1"}

func main() {
	base := flag.String("gateway", env("AI_PREF_EVAL_GATEWAY", "http://127.0.0.1:18080"), "gateway BFF")
	flag.Parse()
	token, err := login(*base)
	if err != nil {
		fatal(err)
	}
	if err := clearPrefs(*base, token); err != nil {
		fatal(err)
	}
	without, err := chat(*base, token, "pref-without-"+fmt.Sprint(time.Now().UnixNano()))
	if err != nil {
		fatal(err)
	}
	if err := setPrefs(*base, token); err != nil {
		fatal(err)
	}
	with, err := chat(*base, token, "pref-with-"+fmt.Sprint(time.Now().UnixNano()))
	if err != nil {
		fatal(err)
	}
	_ = clearPrefs(*base, token)
	fw, okw := firstProduct(with)
	fn, okn := firstProduct(without)
	fmt.Printf("without first=%q (%v)\nwith first=%q (%v)\n", fn, okn, fw, okw)
	if !okw || fw != "云舟 M3" {
		fmt.Println("FAIL: preference did not make 云舟 M3 the first recommendation")
		os.Exit(1)
	}
	if okn && fn == "云舟 M3" {
		fmt.Println("WARN: baseline also starts with 云舟 M3; benchmark not discriminating")
	}
	fmt.Println("PASS: preference changed the leading recommendation")
}

func firstProduct(text string) (string, bool) {
	best, pos := "", len(text)+1
	for _, p := range products {
		if i := strings.Index(text, p); i >= 0 && i < pos {
			best, pos = p, i
		}
	}
	return best, best != ""
}

func chat(base, token, session string) (string, error) {
	body, _ := json.Marshal(map[string]string{"message": prompt, "session_id": session})
	req, _ := http.NewRequest(http.MethodPost, base+"/api/v1/ai/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("chat http %d", resp.StatusCode)
	}
	var out strings.Builder
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var ev struct {
			Type  string `json:"type"`
			Delta string `json:"delta"`
			Error string `json:"error"`
		}
		if json.Unmarshal([]byte(line[6:]), &ev) != nil {
			continue
		}
		if ev.Type == "error" {
			return "", fmt.Errorf("%s", ev.Error)
		}
		out.WriteString(ev.Delta)
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func setPrefs(base, token string) error {
	body, _ := json.Marshal(map[string]any{"preferred_brands": []string{"云舟"}})
	req, _ := http.NewRequest(http.MethodPut, base+"/api/v1/ai/preferences", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("set prefs http %d", resp.StatusCode)
	}
	return nil
}

func clearPrefs(base, token string) error {
	req, _ := http.NewRequest(http.MethodDelete, base+"/api/v1/ai/preferences", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func login(base string) (string, error) {
	body, _ := json.Marshal(map[string]string{"username": "demo", "password": "user123"})
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
