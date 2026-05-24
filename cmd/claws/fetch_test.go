package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestFetch_ValidatesHTTPSOnly(t *testing.T) {
	for _, u := range []string{
		"http://example.com/x",
		"file:///etc/passwd",
		"ftp://example.com/x",
		"javascript:alert(1)",
		"",
		"://broken",
	} {
		if err := validateFetchURL(u); err == nil {
			t.Errorf("validateFetchURL(%q) should have failed", u)
		}
	}
	for _, u := range []string{
		"https://example.com/x",
		"https://raw.githubusercontent.com/foo/bar/main/baz.sh",
	} {
		if err := validateFetchURL(u); err != nil {
			t.Errorf("validateFetchURL(%q) unexpected error: %v", u, err)
		}
	}
}

func TestFetch_HappyPathWithSha(t *testing.T) {
	body := "echo hello-from-test-server\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)

	// Replace httpFetcher with one that strips the https requirement for
	// the test server (which is http). We test the production HTTPS gate
	// separately via validateFetchURL above.
	prev := httpFetcher
	httpFetcher = func(url string) ([]byte, error) {
		resp, err := http.Get(url)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		b := make([]byte, 4096)
		n, _ := resp.Body.Read(b)
		return b[:n], nil
	}
	defer func() { httpFetcher = prev }()

	// Bypass the validateFetchURL gate by calling httpFetcher directly first
	// to compute the real sha — then use validateFetchURL bypass test below.
	got, err := httpFetcher(srv.URL)
	if err != nil {
		t.Fatalf("fetcher direct call: %v", err)
	}
	expected := hashHex(got)

	// Now exercise fetchResource via our injected fetcher + an https-looking
	// URL that we redirect via the fetcher itself. We can't bypass
	// validateFetchURL cleanly, so test it by faking https in cache lookups.
	httpFetcher = func(url string) ([]byte, error) {
		_ = url // doesn't matter — fetcher returns canned body
		return []byte(body), nil
	}
	bytes2, err := fetchResource("https://example.invalid/x", expected)
	if err != nil {
		t.Fatalf("fetchResource happy path: %v", err)
	}
	if string(bytes2) != body {
		t.Errorf("body mismatch: got %q want %q", string(bytes2), body)
	}
}

func TestFetch_RejectsBadSha(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	prev := httpFetcher
	httpFetcher = func(_ string) ([]byte, error) { return []byte("hello"), nil }
	defer func() { httpFetcher = prev }()

	_, err := fetchResource("https://example.invalid/x", "0000000000000000000000000000000000000000000000000000000000000000")
	if err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Errorf("expected sha256 mismatch error, got: %v", err)
	}
}

func TestFetch_CacheHitSkipsDownload(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	body := "cached body\n"
	sha := hashHex([]byte(body))

	// Pre-seed the cache (cache key = sha when expectedSha is provided)
	if err := os.MkdirAll(cache+"/claws/fetched", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cache+"/claws/fetched/"+sha, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	prev := httpFetcher
	calls := 0
	httpFetcher = func(_ string) ([]byte, error) {
		calls++
		return []byte("DIFFERENT — should not be called"), nil
	}
	defer func() { httpFetcher = prev }()

	got, err := fetchResource("https://example.invalid/c", sha)
	if err != nil {
		t.Fatalf("cache hit fetchResource: %v", err)
	}
	if string(got) != body {
		t.Errorf("cache content mismatch: %q", string(got))
	}
	if calls != 0 {
		t.Errorf("expected 0 fetcher calls (cache hit), got %d", calls)
	}
}
