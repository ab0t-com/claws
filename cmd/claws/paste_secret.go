package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// cmdPasteSecret — `claws paste-secret <name> [--secrets-dir=<path>] [--port=N] [--bind=<addr>] [--timeout=<dur>]`
//
// Bridges phone → server for secret values. Spins up a single-purpose
// HTTP listener with a random URL + 6-digit verification code. User opens
// the URL on their phone, pastes the value (e.g. Telegram bot token from
// BotFather), enters the code, submits. Server writes the file and exits.
//
// Why this exists: non-technical users can't reasonably copy a 46-char
// Telegram bot token from a phone via SSH into an editor. This is the
// least-friction phone→server bridge.
//
// Security model:
//   - URL token is 14 hex chars (56 bits of entropy) — unguessable on a LAN
//   - 6-digit code on terminal must echo from the phone (defends against
//     someone with the URL but not the terminal access)
//   - Single-use: server exits after the first successful paste
//   - 5-minute auto-expire if no paste arrives
//   - HTTP-only — fine for ephemeral local-network paste; URL+code are the secret
//   - Operator can pass --bind=127.0.0.1 to require SSH tunnel
func cmdPasteSecret(args []string) error {
	var name, secretsDir, bindAddr string
	port := 8765
	timeout := 5 * time.Minute

	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(`Usage: claws paste-secret <name> [flags]

Spin up a one-shot HTTP listener that lets you paste a secret value from
your phone (or any other device) into a file on this server. Designed for
the case where you have a token on your phone (BotFather, etc.) and SSH
copy-paste is awkward.

Args:
  <name>                       Filename to write under --secrets-dir
                               (e.g. "telegram.token", "openai.key")

Flags:
  --secrets-dir=<path>         Where to write (default /tmp/claws-secrets)
  --port=<n>                   HTTP listen port (default 8765)
  --bind=<addr>                Bind address (default 0.0.0.0 — LAN-reachable;
                               use 127.0.0.1 to require SSH port-forward)
  --timeout=<duration>         How long to listen (default 5m)

Examples:
  claws paste-secret telegram.token
  claws paste-secret openai.key --secrets-dir=/etc/claws/secrets
  claws paste-secret slack.bot-token --bind=127.0.0.1 --port=9000`)
		return nil
	}

	name = args[0]
	for _, a := range args[1:] {
		switch {
		case strings.HasPrefix(a, "--secrets-dir="):
			secretsDir = strings.TrimPrefix(a, "--secrets-dir=")
		case strings.HasPrefix(a, "--port="):
			fmt.Sscanf(strings.TrimPrefix(a, "--port="), "%d", &port)
		case strings.HasPrefix(a, "--bind="):
			bindAddr = strings.TrimPrefix(a, "--bind=")
		case strings.HasPrefix(a, "--timeout="):
			d, err := time.ParseDuration(strings.TrimPrefix(a, "--timeout="))
			if err != nil {
				return errorf("invalid --timeout: %v", err)
			}
			timeout = d
		}
	}

	// Reject path separators and traversal patterns; single dots in filenames
	// (e.g. openai.key, telegram.token) are fine.
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") || name == "" {
		return errorf("invalid name %q (no slashes, no path traversal)", name)
	}
	if secretsDir == "" {
		secretsDir = "/tmp/claws-secrets"
	}
	if bindAddr == "" {
		bindAddr = "0.0.0.0"
	}
	if err := os.MkdirAll(secretsDir, 0700); err != nil {
		return errorf("create secrets dir %s: %v", secretsDir, err)
	}

	urlToken := randomHex(7)               // 14 hex chars
	verifyCode := fmt.Sprintf("%06d", randomInt6()) // "417302"
	codePretty := verifyCode[:3] + "-" + verifyCode[3:]

	// Discover a reachable IP for the URL hint. Fall back to "<your-host>".
	hostHint := guessLANIP()
	if hostHint == "" {
		hostHint = "<your-host>"
	}

	const (
		bold  = "\033[1m"
		green = "\033[0;32m"
		gold  = "\033[0;33m"
		dim   = "\033[0;90m"
		nc    = "\033[0m"
	)

	fmt.Printf("%sclaws paste-secret%s — bridging phone → %s/%s\n\n", bold, nc, secretsDir, name)
	fmt.Printf("  %sOpen on your phone:%s\n", bold, nc)
	fmt.Printf("      %shttp://%s:%d/%s%s\n", green, hostHint, port, urlToken, nc)
	fmt.Println()
	fmt.Printf("  %sEnter this code on the page:%s\n", bold, nc)
	fmt.Printf("      %s%s%s\n", green, codePretty, nc)
	fmt.Println()
	fmt.Printf("  %sListening on %s:%d for %s ... (Ctrl-C to cancel)%s\n",
		dim, bindAddr, port, timeout, nc)
	fmt.Println()

	// Channels to coordinate exit between handler + timeout.
	done := make(chan error, 1)
	var once sync.Once
	finish := func(err error) { once.Do(func() { done <- err }) }

	mux := http.NewServeMux()
	mux.HandleFunc("/"+urlToken, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, pasteFormHTML(html.EscapeString(name), urlToken))
		case http.MethodPost:
			if err := r.ParseForm(); err != nil {
				http.Error(w, "bad form", http.StatusBadRequest)
				return
			}
			gotCode := strings.ReplaceAll(strings.ReplaceAll(r.PostFormValue("code"), "-", ""), " ", "")
			value := strings.TrimSpace(r.PostFormValue("value"))
			if gotCode != verifyCode {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, pasteResultHTML("Code didn't match. Check the terminal and try again.", false))
				return
			}
			if value == "" {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, pasteResultHTML("Value is empty. Paste a token and submit.", false))
				return
			}
			target := filepath.Join(secretsDir, name)
			if err := os.WriteFile(target, []byte(value+"\n"), 0600); err != nil {
				http.Error(w, "write failed: "+err.Error(), http.StatusInternalServerError)
				finish(err)
				return
			}
			fmt.Fprint(w, pasteResultHTML("Saved. You can close this tab.", true))
			fmt.Printf("\n  %s✓ Received %s — written to %s%s\n\n", green, name, target, nc)
			finish(nil)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	// Any other path: 404 with no leak about the right token.
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	})

	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", bindAddr, port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			finish(err)
		}
	}()

	// Timeout watchdog.
	go func() {
		<-time.After(timeout)
		finish(errorf("timed out after %s — no paste received", timeout))
	}()

	err := <-done
	// Best-effort shutdown.
	ctx := newShortCtx(2 * time.Second)
	_ = srv.Shutdown(ctx)
	return err
}

// randomHex returns N hex chars (N/2 bytes of entropy).
func randomHex(charLen int) string {
	b := make([]byte, charLen/2+1)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:charLen]
}

// randomInt6 returns a random 6-digit number (000000-999999).
func randomInt6() int {
	var b [4]byte
	_, _ = rand.Read(b[:])
	n := int(b[0])<<24 | int(b[1])<<16 | int(b[2])<<8 | int(b[3])
	if n < 0 {
		n = -n
	}
	return n % 1000000
}

// guessLANIP returns a likely LAN IP for the URL hint. Falls back to "".
// Walks interfaces, returns the first non-loopback IPv4.
func guessLANIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok || ipNet.IP.To4() == nil {
				continue
			}
			return ipNet.IP.String()
		}
	}
	return ""
}

// newShortCtx is a minimal context wrapper for graceful shutdown.
// Avoids adding context/cancel dance; the server shutdown only needs a
// deadline. Kept local to this file to limit dependency surface.
type shortCtx struct{ d time.Duration }

func newShortCtx(d time.Duration) *shortCtx                       { return &shortCtx{d: d} }
func (c *shortCtx) Deadline() (time.Time, bool)                   { return time.Now().Add(c.d), true }
func (c *shortCtx) Done() <-chan struct{}                         { ch := make(chan struct{}); time.AfterFunc(c.d, func() { close(ch) }); return ch }
func (c *shortCtx) Err() error                                    { return nil }
func (c *shortCtx) Value(key interface{}) interface{}             { return nil }

// pasteFormHTML returns the mobile-friendly single-page form.
func pasteFormHTML(name, urlToken string) string {
	return `<!doctype html>
<html><head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>claws paste-secret — ` + name + `</title>
<style>
  body { font-family: -apple-system, BlinkMacSystemFont, system-ui, sans-serif;
         margin: 0; padding: 24px; background: #fafafa; color: #1a1714; max-width: 540px; }
  h1   { font-family: Georgia, serif; font-weight: 400; font-size: 1.6rem; margin: 0 0 16px 0; }
  p    { color: #5a5550; margin: 0 0 18px 0; line-height: 1.55; }
  label { display: block; font-weight: 600; margin: 18px 0 6px; font-size: 0.95rem; }
  input, textarea {
    width: 100%; padding: 14px; font-size: 1rem; font-family: ui-monospace, "SF Mono", monospace;
    border: 2px solid #edeae4; border-radius: 8px; box-sizing: border-box;
  }
  textarea { min-height: 110px; resize: vertical; }
  button {
    margin-top: 24px; width: 100%; padding: 16px; font-size: 1rem; font-weight: 600;
    background: #e8634a; color: white; border: 0; border-radius: 8px;
  }
  .hint { font-size: 0.85rem; color: #8a8580; margin-top: 4px; }
  .code { letter-spacing: 0.12em; font-size: 1.05rem; text-align: center; }
</style>
</head>
<body>
<h1>Paste your ` + name + `</h1>
<p>Pasted from BotFather, OpenAI, or whatever app gave you the token.</p>
<form method="POST" action="/` + urlToken + `">
  <label for="code">Code from the terminal</label>
  <input id="code" name="code" class="code" placeholder="123-456" autocomplete="off" autocapitalize="off" required>
  <p class="hint">The 6 digits shown where you ran <code>claws paste-secret</code>.</p>

  <label for="value">Paste the secret value</label>
  <textarea id="value" name="value" placeholder="Paste here" autocomplete="off" autocapitalize="off" spellcheck="false" required></textarea>
  <p class="hint">It'll be written to a file on the server. Single use — close this tab after.</p>

  <button type="submit">Save secret</button>
</form>
</body></html>`
}

func pasteResultHTML(msg string, ok bool) string {
	color := "#e8634a"
	if ok {
		color = "#7da67d"
	}
	return `<!doctype html>
<html><head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>claws paste-secret</title>
<style>
  body { font-family: -apple-system, system-ui, sans-serif; padding: 40px 24px;
         text-align: center; background: #fafafa; color: #1a1714; }
  .mark { font-size: 4rem; color: ` + color + `; margin-bottom: 16px; }
  p { font-size: 1.1rem; color: #3a3632; }
  a { color: #e8634a; }
</style>
</head><body>
<div class="mark">` + map[bool]string{true: "✓", false: "✗"}[ok] + `</div>
<p>` + html.EscapeString(msg) + `</p>
` + map[bool]string{true: "", false: `<p><a href="javascript:history.back()">← Try again</a></p>`}[ok] + `
</body></html>`
}
