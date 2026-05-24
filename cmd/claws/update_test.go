package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompareSemver(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v1.6.4", "v1.6.4", 0},
		{"v1.6.4", "v1.6.5", -1},
		{"v1.6.5", "v1.6.4", 1},
		{"v1.6.4", "v1.7.0", -1},
		{"v2.0.0", "v1.99.99", 1},
		{"1.6.4", "v1.6.4", 0},       // missing v prefix is ok
		{"v1.6.5", "v1.6.5-dirty", -1}, // tag < dev build after that tag
		{"v1.6.5-dirty", "v1.6.5", 1},
		{"v1.6.5-1-gabc", "v1.6.5-2-gdef", -1}, // lexicographic for dev builds
		{"v1.6.5-dirty", "v1.6.5-dirty", 0},
	}
	for _, c := range cases {
		got := compareSemver(c.a, c.b)
		if got != c.want {
			t.Errorf("compareSemver(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestSplitSemver(t *testing.T) {
	cases := []struct {
		in       string
		wantNums [3]int
		wantSuf  string
	}{
		{"v1.6.4", [3]int{1, 6, 4}, ""},
		{"1.6.4", [3]int{1, 6, 4}, ""},
		{"v1.6.5-dirty", [3]int{1, 6, 5}, "dirty"},
		{"v2.10.100", [3]int{2, 10, 100}, ""},
		{"v1.6.6-1-gabc123", [3]int{1, 6, 6}, "1-gabc123"},
	}
	for _, c := range cases {
		nums, suf := splitSemver(c.in)
		if nums != c.wantNums || suf != c.wantSuf {
			t.Errorf("splitSemver(%q) = (%v, %q), want (%v, %q)",
				c.in, nums, suf, c.wantNums, c.wantSuf)
		}
	}
}

func TestLookupChecksum(t *testing.T) {
	dir := t.TempDir()
	sumPath := filepath.Join(dir, "SHA256SUMS")
	body := `abc123  claws-v1.6.5-linux-amd64.tar.gz
def456  claws-v1.6.5-darwin-arm64.tar.gz
`
	if err := os.WriteFile(sumPath, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := lookupChecksum(sumPath, "claws-v1.6.5-linux-amd64.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if got != "abc123" {
		t.Errorf("got %q want abc123", got)
	}
	if _, err := lookupChecksum(sumPath, "no-such-tarball.tar.gz"); err == nil {
		t.Error("expected error for missing tarball")
	}
}

func TestExtractClawsBinary(t *testing.T) {
	// Build an in-memory tar.gz containing a fake 'claws' binary nested
	// under a release-style directory (claws-v1.0.0-linux-amd64/claws).
	dir := t.TempDir()
	tarPath := filepath.Join(dir, "fake.tar.gz")

	buf := &bytes.Buffer{}
	gz := gzip.NewWriter(buf)
	tw := tar.NewWriter(gz)
	// A junk file we should skip.
	writeTarFile(t, tw, "claws-v1.0.0-linux-amd64/README", []byte("ignore me"))
	// The real binary.
	writeTarFile(t, tw, "claws-v1.0.0-linux-amd64/claws", []byte("#!/bin/sh\necho fake-claws"))
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tarPath, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := extractClawsBinary(tarPath, dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("fake-claws")) {
		t.Errorf("extracted bytes don't contain expected payload: %q", got)
	}
	fi, _ := os.Stat(out)
	if fi.Mode().Perm()&0100 == 0 {
		t.Errorf("extracted binary should be executable, got mode %v", fi.Mode())
	}
}

func TestExtractClawsBinary_Missing(t *testing.T) {
	dir := t.TempDir()
	tarPath := filepath.Join(dir, "nofile.tar.gz")
	buf := &bytes.Buffer{}
	gz := gzip.NewWriter(buf)
	tw := tar.NewWriter(gz)
	writeTarFile(t, tw, "some-other-thing", []byte("nope"))
	tw.Close()
	gz.Close()
	os.WriteFile(tarPath, buf.Bytes(), 0644)

	if _, err := extractClawsBinary(tarPath, dir); err == nil {
		t.Error("expected error when no claws binary in archive")
	}
}

func TestSha256OfFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x")
	os.WriteFile(p, []byte("hello"), 0644)
	got, err := sha256OfFile(p)
	if err != nil {
		t.Fatal(err)
	}
	// echo -n hello | sha256sum
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestFetchLatestReleaseVersion(t *testing.T) {
	// Spin up a fake HTTP server that serves a VERSION file.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/VERSION") {
			_, _ = w.Write([]byte("v9.9.9\n"))
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	// Reuse the helper directly to avoid mucking with the const.
	body, err := httpGetBytes(srv.URL + "/VERSION")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(body)) != "v9.9.9" {
		t.Errorf("got %q want v9.9.9", string(body))
	}
}

// writeTarFile is a tiny helper for building test archives.
func writeTarFile(t *testing.T, tw *tar.Writer, name string, body []byte) {
	t.Helper()
	hdr := &tar.Header{
		Name:     name,
		Mode:     0755,
		Size:     int64(len(body)),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
}
