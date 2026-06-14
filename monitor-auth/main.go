package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"html/template"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	monitorUser = os.Getenv("MONITOR_USER")
	monitorPass = os.Getenv("MONITOR_PASSWORD")
	netdataURL  = envOr("NETDATA_URL", "http://netdata:19999")
	addr        = envOr("ADDR", ":8080")
	sessionTTL  = 7 * 24 * time.Hour
	signingKey  = deriveKey(monitorUser, monitorPass)
)

const cookieName = "monitor_session"

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// deriveKey ties the session secret to the credentials, so sessions survive
// redeploys but are invalidated automatically when the login or password changes.
func deriveKey(user, pass string) []byte {
	sum := sha256.Sum256([]byte(user + "\x00" + pass))
	return sum[:]
}

func sign(value string) string {
	mac := hmac.New(sha256.New, signingKey)
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

// issueToken returns a "<expiry>|<signature>" token valid for sessionTTL.
func issueToken() string {
	exp := strconv.FormatInt(time.Now().Add(sessionTTL).Unix(), 10)
	return exp + "|" + sign(exp)
}

func validToken(token string) bool {
	parts := strings.SplitN(token, "|", 2)
	if len(parts) != 2 {
		return false
	}
	exp, sig := parts[0], parts[1]
	if subtle.ConstantTimeCompare([]byte(sig), []byte(sign(exp))) != 1 {
		return false
	}
	ts, err := strconv.ParseInt(exp, 10, 64)
	if err != nil {
		return false
	}
	return time.Now().Unix() < ts
}

func validCredentials(user, pass string) bool {
	if monitorUser == "" || monitorPass == "" {
		return false
	}
	userOK := subtle.ConstantTimeCompare([]byte(user), []byte(monitorUser)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(pass), []byte(monitorPass)) == 1
	return userOK && passOK
}

func authenticated(r *http.Request) bool {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}
	return validToken(c.Value)
}

var loginPage = template.Must(template.New("login").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Monitoring · sign in</title>
<style>
  :root { color-scheme: dark; }
  * { box-sizing: border-box; }
  body {
    margin: 0; min-height: 100vh; display: grid; place-items: center;
    font: 15px/1.5 system-ui, -apple-system, Segoe UI, Roboto, sans-serif;
    background: radial-gradient(120% 120% at 50% 0%, #1b2230 0%, #0c0f16 60%);
    color: #e7ecf3;
  }
  .card {
    width: min(360px, 92vw); padding: 32px 28px; border-radius: 16px;
    background: #131824; border: 1px solid #232c3d;
    box-shadow: 0 20px 60px rgba(0,0,0,.45);
  }
  h1 { margin: 0 0 4px; font-size: 20px; }
  p.sub { margin: 0 0 24px; color: #8b97ab; font-size: 13px; }
  label { display: block; margin: 0 0 6px; font-size: 13px; color: #b3bccc; }
  input {
    width: 100%; padding: 11px 13px; margin-bottom: 16px; border-radius: 9px;
    border: 1px solid #2c3648; background: #0d111a;
    color: #e7ecf3; font-size: 14px; outline: none;
  }
  input:focus { border-color: #3b82f6; }
  button {
    width: 100%; padding: 11px; border: 0; border-radius: 9px; cursor: pointer;
    background: #3b82f6; color: #fff; font-size: 14px; font-weight: 600;
  }
  button:hover { background: #2f6fe0; }
  .err {
    margin: 0 0 16px; padding: 10px 12px; border-radius: 9px; font-size: 13px;
    background: #3a1620; border: 1px solid #5a2230; color: #f3a4b2;
  }
</style>
</head>
<body>
  <form class="card" method="post" action="/login">
    <h1>Monitoring</h1>
    <p class="sub">Sign in to view server metrics</p>
    {{if .Error}}<p class="err">{{.Error}}</p>{{end}}
    <label for="username">Username</label>
    <input id="username" name="username" autocomplete="username" autofocus required>
    <label for="password">Password</label>
    <input id="password" name="password" type="password" autocomplete="current-password" required>
    <button type="submit">Sign in</button>
  </form>
</body>
</html>`))

func renderLogin(w http.ResponseWriter, status int, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = loginPage.Execute(w, struct{ Error string }{errMsg})
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if authenticated(r) {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		renderLogin(w, http.StatusOK, "")
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !validCredentials(r.FormValue("username"), r.FormValue("password")) {
		renderLogin(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    issueToken(),
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func main() {
	target, err := url.Parse(netdataURL)
	if err != nil {
		log.Fatalf("invalid NETDATA_URL %q: %v", netdataURL, err)
	}
	if monitorUser == "" || monitorPass == "" {
		log.Println("WARNING: MONITOR_USER or MONITOR_PASSWORD is empty; all logins will be rejected")
	}
	proxy := httputil.NewSingleHostReverseProxy(target)

	mux := http.NewServeMux()
	mux.HandleFunc("/login", loginHandler)
	mux.HandleFunc("/logout", logoutHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !authenticated(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		proxy.ServeHTTP(w, r)
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("monitor-auth listening on %s, proxying to %s", addr, netdataURL)
	log.Fatal(srv.ListenAndServe())
}
