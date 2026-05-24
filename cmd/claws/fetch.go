package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// fetchTimeout is the per-request deadline for URL-loaded resources.
var fetchTimeout = 30 * time.Second

// httpFetcher is the function used to fetch URLs. Tests inject a fake.
var httpFetcher = defaultHTTPFetcher

// fetchCacheDir resolves the cache directory for downloaded resources.
// ~/.cache/claws/fetched/ by default; respects $XDG_CACHE_HOME.
func fetchCacheDir() string {
	if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
		return filepath.Join(v, "claws", "fetched")
	}
	if home, _ := os.UserHomeDir(); home != "" {
		return filepath.Join(home, ".cache", "claws", "fetched")
	}
	return filepath.Join(os.TempDir(), "claws-fetched")
}

// validateFetchURL enforces our security rules for URL-loaded resources.
//   - https only (no http://, no file://, no anything else)
//   - well-formed URL
func validateFetchURL(raw string) error {
	if raw == "" {
		return errorf("empty fromUrl")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return errorf("invalid fromUrl %q: %v", raw, err)
	}
	if u.Scheme != "https" {
		return errorf("fromUrl must use https:// (got %q)", u.Scheme)
	}
	if u.Host == "" {
		return errorf("fromUrl missing host: %q", raw)
	}
	return nil
}

// fetchResource downloads `rawURL`, verifies SHA256 if `expectedSha` is
// non-empty, caches the result, and returns the bytes.
//
// Cache key:
//   - With sha256 declared: <expectedSha>      (content-addressed; very safe)
//   - Without:              urlHash(<rawURL>)   (URL-addressed; warned at apply-time)
func fetchResource(rawURL, expectedSha string) ([]byte, error) {
	if err := validateFetchURL(rawURL); err != nil {
		return nil, err
	}
	cacheDir := fetchCacheDir()
	_ = os.MkdirAll(cacheDir, 0755)

	cacheKey := expectedSha
	if cacheKey == "" {
		h := sha256.Sum256([]byte(rawURL))
		cacheKey = "url-" + hex.EncodeToString(h[:])[:32]
	}
	cachePath := filepath.Join(cacheDir, cacheKey)

	// Cache hit
	if data, err := os.ReadFile(cachePath); err == nil {
		if expectedSha != "" {
			if got := hashHex(data); got != expectedSha {
				// Cache poisoned somehow — delete and re-fetch
				_ = os.Remove(cachePath)
			} else {
				return data, nil
			}
		} else {
			return data, nil
		}
	}

	// Fetch
	data, err := httpFetcher(rawURL)
	if err != nil {
		return nil, errorf("fetch %s: %v", rawURL, err)
	}

	// SHA256 verify
	if expectedSha != "" {
		got := hashHex(data)
		if got != strings.ToLower(strings.TrimSpace(expectedSha)) {
			return nil, errorf("sha256 mismatch for %s\n  expected: %s\n  actual:   %s",
				rawURL, expectedSha, got)
		}
	}

	// Persist cache (best-effort)
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		// Not fatal — return the bytes anyway
		fmt.Fprintf(os.Stderr, "  ! warning: cache write failed: %v\n", err)
	}
	return data, nil
}

func hashHex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// defaultHTTPFetcher is the production implementation. Tests replace it
// with an httptest.Server-backed function.
func defaultHTTPFetcher(rawURL string) ([]byte, error) {
	client := &http.Client{Timeout: fetchTimeout}
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errorf("HTTP %d from %s", resp.StatusCode, rawURL)
	}
	const maxBody = 4 * 1024 * 1024 // 4 MiB cap
	return io.ReadAll(io.LimitReader(resp.Body, maxBody))
}
