package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFetchTarball_HappyPath — minimal server serves a payload + sidecar;
// fetchTarball downloads it and returns the correct sha.
func TestFetchTarball_HappyPath(t *testing.T) {
	payload := []byte("hello, openclaw")
	expected := sha256.Sum256(payload)
	expectedHex := hex.EncodeToString(expected[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/img.tar.gz":
			w.Write(payload)
		case "/img.tar.gz.sha256":
			fmt.Fprintf(w, "%s  img.tar.gz\n", expectedHex)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "img.tar.gz")
	got, err := fetchTarball(srv.URL+"/img.tar.gz", dest)
	if err != nil {
		t.Fatalf("fetchTarball: %v", err)
	}
	if got != expectedHex {
		t.Errorf("hash mismatch: got %s, want %s", got, expectedHex)
	}
	// Verify file landed on disk with right content.
	read, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(read) != string(payload) {
		t.Errorf("file content mismatch: got %q, want %q", read, payload)
	}
}

// TestFetchExpectedSha256 — sidecar parsing handles standard sha256sum format.
func TestFetchExpectedSha256(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
		wErr bool
	}{
		{"standard sha256sum format", "4704e2e254275934d793447f7fb8475d4fe4b694b50ddcdce388f553a5aeb3a9  openclaw.tar.gz\n", "4704e2e254275934d793447f7fb8475d4fe4b694b50ddcdce388f553a5aeb3a9", false},
		{"no filename trailing", "4704e2e254275934d793447f7fb8475d4fe4b694b50ddcdce388f553a5aeb3a9\n", "4704e2e254275934d793447f7fb8475d4fe4b694b50ddcdce388f553a5aeb3a9", false},
		{"empty body", "", "", true},
		{"too short", "abc123", "", true},
		{"not hex length 64", "not-a-hex-string-and-also-not-64-characters-long-this-is-bad-no-good", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(c.body))
			}))
			defer srv.Close()
			got, err := fetchExpectedSha256(srv.URL)
			if c.wErr && err == nil {
				t.Errorf("want error, got nil")
			}
			if !c.wErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// TestFetchExpectedSha256_404 — when the sidecar doesn't exist, refuse.
func TestFetchExpectedSha256_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()
	_, err := fetchExpectedSha256(srv.URL + "/missing.sha256")
	if err == nil {
		t.Error("want error on 404, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("want error mentioning HTTP 404, got %v", err)
	}
}

// TestFetchTarball_HTTP404 — download itself fails on 404.
func TestFetchTarball_HTTP404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()
	_, err := fetchTarball(srv.URL+"/missing.tar.gz", filepath.Join(t.TempDir(), "out"))
	if err == nil {
		t.Error("want error on 404, got nil")
	}
}

// TestBootstrapFromTarball_ShaMismatch — when sidecar reports a hash that
// doesn't match the downloaded payload, refuse and DON'T docker load.
func TestBootstrapFromTarball_ShaMismatch(t *testing.T) {
	payload := []byte("real payload")
	// Deliberately publish a hash of OTHER bytes.
	wrongHash := sha256.Sum256([]byte("wrong payload"))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, ".sha256"):
			fmt.Fprintf(w, "%s  img.tar.gz\n", hex.EncodeToString(wrongHash[:]))
		default:
			w.Write(payload)
		}
	}))
	defer srv.Close()

	err := bootstrapFromTarball(srv.URL + "/img.tar.gz")
	if err == nil {
		t.Fatal("want sha256 mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Errorf("want sha256 mismatch error, got %v", err)
	}
}

// TestRequireDiskFree — pre-flight check returns nil for sane host, error
// for impossible requirement. (Can't easily test the negative-path on this
// host without injecting a stat fixture, but verify the threshold check
// doesn't false-positive when free space is large.)
func TestRequireDiskFree(t *testing.T) {
	if err := requireDiskFree("/", 1); err != nil {
		t.Errorf("want nil with 1-byte requirement, got %v", err)
	}
	// Requirement of an impossibly large amount → error.
	if err := requireDiskFree("/", 1<<63); err == nil {
		t.Error("want error with 8 EB requirement, got nil")
	}
}

// TestDefaultTarballURL_LooksRight — guard against accidentally clobbering
// the constant with a placeholder value during refactors.
func TestDefaultTarballURL_LooksRight(t *testing.T) {
	if !strings.HasPrefix(defaultTarballURL, "https://") {
		t.Errorf("defaultTarballURL must be https, got %q", defaultTarballURL)
	}
	if !strings.HasSuffix(defaultTarballURL, ".tar.gz") {
		t.Errorf("defaultTarballURL must end .tar.gz, got %q", defaultTarballURL)
	}
}

// TestPrintPostBootstrapNextSteps_NoPanic — the next-steps printer must not
// panic when paths can't enumerate instances. Used after every success path
// in cmdImageBootstrap; if it crashes we drop the entire success message.
func TestPrintPostBootstrapNextSteps_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printPostBootstrapNextSteps panicked: %v", r)
		}
	}()
	printPostBootstrapNextSteps()
}
