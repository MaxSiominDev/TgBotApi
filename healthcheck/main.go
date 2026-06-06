package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var (
	testBotToken   = os.Getenv("TEST_BOT_TOKEN")
	testChatID     = os.Getenv("TEST_CHAT_ID")
	healthToken    = os.Getenv("HEALTH_TOKEN")
	composeProject = envOr("COMPOSE_PROJECT", "telegram-bot-api")
	localTgAPI     = envOr("LOCAL_TG_API", "http://telegram-bot-api:8081")
	cloudTgAPI     = envOr("CLOUD_TG_API", "https://api.telegram.org")
	dockerSock     = envOr("DOCKER_SOCK", "/var/run/docker.sock")
)

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

var dockerClient = &http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", dockerSock)
		},
	},
}

type checkResult struct {
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

func ok(detail string) checkResult         { return checkResult{OK: true, Detail: detail} }
func fail(detail string) checkResult       { return checkResult{OK: false, Detail: detail} }
func failf(f string, a ...any) checkResult { return fail(fmt.Sprintf(f, a...)) }

func checkContainers() checkResult {
	filters, _ := json.Marshal(map[string][]string{
		"label": {"com.docker.compose.project=" + composeProject},
	})
	u := "http://unix/v1.41/containers/json?all=true&filters=" + url.QueryEscape(string(filters))
	resp, err := dockerClient.Get(u)
	if err != nil {
		return failf("docker api error: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return failf("docker api status %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	var items []struct {
		Names []string `json:"Names"`
		State string   `json:"State"`
	}
	if err := json.Unmarshal(body, &items); err != nil {
		return failf("docker api decode: %v", err)
	}
	if len(items) == 0 {
		return fail("no containers found for project")
	}
	var notRunning []string
	for _, c := range items {
		if c.State != "running" {
			name := "?"
			if len(c.Names) > 0 {
				name = c.Names[0]
			}
			notRunning = append(notRunning, fmt.Sprintf("%s=%s", name, c.State))
		}
	}
	if len(notRunning) > 0 {
		return failf("not running: %s", strings.Join(notRunning, ", "))
	}
	return ok(fmt.Sprintf("%d containers running", len(items)))
}

func checkTelegramReachable() checkResult {
	c := &http.Client{Timeout: 10 * time.Second}
	resp, err := c.Get(strings.TrimRight(cloudTgAPI, "/") + "/bot0:invalid/getMe")
	if err != nil {
		return failf("unreachable: %v", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return ok(fmt.Sprintf("HTTP %d", resp.StatusCode))
}

func checkSendMessage(domain string) checkResult {
	if testBotToken == "" || testChatID == "" {
		return fail("TEST_BOT_TOKEN or TEST_CHAT_ID not set")
	}
	if domain == "" {
		domain = "unknown"
	}
	form := url.Values{
		"chat_id":              {testChatID},
		"text":                 {domain + ": healthcheck ok"},
		"disable_notification": {"true"},
	}
	c := &http.Client{Timeout: 15 * time.Second}
	resp, err := c.PostForm(
		fmt.Sprintf("%s/bot%s/sendMessage", strings.TrimRight(localTgAPI, "/"), testBotToken),
		form,
	)
	if err != nil {
		return failf("error: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var r struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	_ = json.Unmarshal(body, &r)
	if resp.StatusCode != 200 || !r.OK {
		return failf("HTTP %d ok=%v: %s", resp.StatusCode, r.OK, truncate(string(body), 200))
	}
	return ok("sendMessage ok")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func writeResult(w http.ResponseWriter, res checkResult) {
	status := http.StatusServiceUnavailable
	if res.OK {
		status = http.StatusOK
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(res)
}

func requestDomain(r *http.Request) string {
	if d := r.Header.Get("X-Forwarded-Host"); d != "" {
		return d
	}
	return r.Host
}

func authed(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if healthToken == "" || r.Header.Get("X-Api-Token") != healthToken {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		h(w, r)
	}
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health/containers", func(w http.ResponseWriter, r *http.Request) {
		writeResult(w, checkContainers())
	})
	mux.HandleFunc("/health/telegram", authed(func(w http.ResponseWriter, r *http.Request) {
		writeResult(w, checkTelegramReachable())
	}))
	mux.HandleFunc("/health/send-message", authed(func(w http.ResponseWriter, r *http.Request) {
		writeResult(w, checkSendMessage(requestDomain(r)))
	}))
	srv := &http.Server{
		Addr:              envOr("ADDR", ":8080"),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}
