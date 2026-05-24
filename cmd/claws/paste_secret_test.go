package main

import (
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// pasteSecretRun starts cmdPasteSecret in a goroutine with a fast timeout,
// reaches into the output to extract the URL token + code, and returns
// them so a test can drive the HTTP form.
//
// Approach: we can't easily extract the URL token from inside the
// running goroutine (it logs to stdout). We instead create a tiny wrapper
// that listens, captures, and matches behavior. For test-friendliness,
// the cleanest path is to test the helpers directly (randomHex,
// randomInt6) + spin up an instance and POST.
//
// We'll spin up the real server via cmdPasteSecret, capture stdout via
// a pipe, parse the URL + code, drive POSTs through net.Dial.
func TestPasteSecret_RandomHexAndInt(t *testing.T) {
	a := randomHex(7)
	b := randomHex(7)
	if a == b {
		t.Errorf("randomHex collision (or seed bug): %s == %s", a, b)
	}
	if len(a) != 7 {
		t.Errorf("randomHex length: %d", len(a))
	}
	matched, _ := regexp.MatchString(`^[a-f0-9]{7}$`, a)
	if !matched {
		t.Errorf("randomHex shape wrong: %q", a)
	}
	for i := 0; i < 100; i++ {
		n := randomInt6()
		if n < 0 || n >= 1000000 {
			t.Errorf("randomInt6 out of range: %d", n)
		}
	}
}

func TestPasteSecret_RejectsInvalidNames(t *testing.T) {
	for _, bad := range []string{
		"",
		"foo/bar",
		"../etc/passwd",
		"..",
		"a/b/c",
	} {
		err := cmdPasteSecret([]string{bad, "--bind=127.0.0.1", "--port=0", "--timeout=100ms"})
		if err == nil {
			t.Errorf("expected rejection for name %q", bad)
		}
	}
}

// End-to-end: spin up the server, drive it via raw HTTP, verify the
// happy path, wrong-code path, single-use semantics.
func TestPasteSecret_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	port, err := freePort()
	if err != nil {
		t.Fatal(err)
	}

	// Capture stdout to read the URL + code.
	oldOut := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldOut }()

	done := make(chan error, 1)
	go func() {
		done <- cmdPasteSecret([]string{
			"e2e.token",
			"--secrets-dir=" + dir,
			"--bind=127.0.0.1",
			"--port=" + itoa(port),
			"--timeout=3s",
		})
	}()

	// Read the printed URL + code from stdout. The server logs them on startup.
	urlTok, code := readURLAndCode(t, r)
	_ = oldOut
	os.Stdout = oldOut // restore so any further logging doesn't block
	_ = w.Close()

	base := "http://127.0.0.1:" + itoa(port)
	// Wait briefly for the listener to come up (goroutine race).
	var resp string
	for i := 0; i < 20; i++ {
		resp, err = httpPostForm(base+"/"+urlTok, map[string]string{"code": "000000", "value": "x"})
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("wrong-code POST after retries: %v", err)
	}
	// Apostrophe in "didn't" gets HTML-escaped (&#39;) — check for the
	// surrounding words instead.
	if !strings.Contains(resp, "Code") || !strings.Contains(resp, "match") {
		t.Errorf("wrong-code response missing Code/match: %s", resp)
	}

	// File should still not exist.
	if _, err := os.Stat(filepath.Join(dir, "e2e.token")); err == nil {
		t.Errorf("file written on wrong-code attempt — should be rejected")
	}

	// Right code → write + exit.
	resp, err = httpPostForm(base+"/"+urlTok, map[string]string{"code": code, "value": "sk-fake-real"})
	if err != nil {
		t.Fatalf("right-code POST: %v", err)
	}
	if !strings.Contains(resp, "Saved") {
		t.Errorf("right-code response missing 'Saved': %s", resp)
	}

	// Server should exit promptly.
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("server returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server didn't exit after successful paste")
	}

	// File written?
	got, err := os.ReadFile(filepath.Join(dir, "e2e.token"))
	if err != nil {
		t.Fatalf("file not written: %v", err)
	}
	if strings.TrimSpace(string(got)) != "sk-fake-real" {
		t.Errorf("file content wrong: %q", string(got))
	}
}

// --- helpers (kept package-local to avoid bloating shared test infra) ---

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// itoa is a tiny stand-in to avoid importing strconv just for the smoke test.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	if neg {
		d = append([]byte{'-'}, d...)
	}
	return string(d)
}

// readURLAndCode tails the captured stdout for the URL + code lines.
func readURLAndCode(t *testing.T, r *os.File) (string, string) {
	t.Helper()
	buf := make([]byte, 4096)
	deadline := time.Now().Add(2 * time.Second)
	var sofar string
	for time.Now().Before(deadline) {
		_ = r.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, _ := r.Read(buf)
		if n > 0 {
			sofar += string(buf[:n])
		}
		urlRE := regexp.MustCompile(`/([a-f0-9]{7})\b`)
		codeRE := regexp.MustCompile(`([0-9]{3})-([0-9]{3})`)
		if u := urlRE.FindStringSubmatch(sofar); u != nil {
			if c := codeRE.FindStringSubmatch(sofar); c != nil {
				return u[1], c[1] + c[2]
			}
		}
	}
	t.Fatalf("couldn't extract URL+code from server stdout:\n%s", sofar)
	return "", ""
}

func httpPostForm(target string, form map[string]string) (string, error) {
	v := url.Values{}
	for k, val := range form {
		v.Set(k, val)
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.PostForm(target, v)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body), nil
}
