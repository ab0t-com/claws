package main

import (
	"strings"
	"testing"
)

func TestParseSwapSize(t *testing.T) {
	cases := []struct {
		in   string
		want uint64
	}{
		{"", 8 * 1024 * 1024 * 1024},
		{"8g", 8 * 1024 * 1024 * 1024},
		{"4G", 4 * 1024 * 1024 * 1024},
		{"2048m", 2048 * 1024 * 1024},
		{"  2048M  ", 2048 * 1024 * 1024},
		{"1g", 1 * 1024 * 1024 * 1024},
		// Plain numbers default to bytes (1 byte * 1 = 1)
		{"1024", 1024},
		// Invalid → 0 (caller treats as error)
		{"abc", 0},
		{"8gg", 0},
	}
	for _, c := range cases {
		if got := parseSwapSize(c.in); got != c.want {
			t.Errorf("parseSwapSize(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseProcSwapsTotal(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    uint64
	}{
		{
			"empty (no swap configured)",
			"Filename                Type        Size            Used    Priority\n",
			0,
		},
		{
			"single fstab swapfile",
			"Filename                Type        Size            Used    Priority\n" +
				"/swapfile               file        5242876         581768  -2\n",
			5242876 * 1024,
		},
		{
			"multiple swaps summed",
			"Filename                Type        Size            Used    Priority\n" +
				"/dev/sda5               partition   2097148         0       -2\n" +
				"/swap.img               file        4194300         0       -3\n",
			(2097148 + 4194300) * 1024,
		},
		{
			"trailing newline tolerated",
			"Filename Type Size Used Priority\n/swapfile file 1024 0 -1\n\n",
			1024 * 1024,
		},
		{
			"garbage tolerated (skip un-parseable)",
			"Filename Type Size Used Priority\nweird line\n/swap file 2048 0 -1\n",
			2048 * 1024,
		},
	}
	for _, c := range cases {
		got := parseProcSwapsTotal(c.content)
		if got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}

func TestSwapfileActiveIn(t *testing.T) {
	content := "Filename Type Size Used Priority\n" +
		"/tmp/claws-bootstrap.swap file 8589934 0 -2\n" +
		"/swapfile file 5242876 0 -3\n"

	cases := []struct {
		path string
		want bool
	}{
		{"/tmp/claws-bootstrap.swap", true},
		{"/swapfile", true},
		{"/no-such-swap", false},
		{"", false},
		// Path is a prefix of an active swap but not exact — should be false
		{"/tmp", false},
	}
	for _, c := range cases {
		got := swapfileActiveIn(content, c.path)
		if got != c.want {
			t.Errorf("swapfileActiveIn(%q) = %v, want %v", c.path, got, c.want)
		}
	}

	// Header-only (no swap) → false for anything
	if swapfileActiveIn("Filename Type Size Used Priority\n", "/anything") {
		t.Error("header-only content should not report anything as active")
	}

	// Empty content → false
	if swapfileActiveIn("", "/tmp/x") {
		t.Error("empty content should return false")
	}

	// Sanity: nothing weird with whitespace
	if !swapfileActiveIn(strings.Join([]string{
		"Filename Type Size Used Priority",
		"/tmp/x  file  1024  0  -1",
	}, "\n"), "/tmp/x") {
		t.Error("expected /tmp/x to match a tab-separated entry")
	}
}

func TestMountFstypeForPath(t *testing.T) {
	// Realistic mini /proc/mounts (truncated). Devices and options
	// don't matter for our test; only field-1 (mountpoint) and field-2
	// (fstype).
	mounts := strings.Join([]string{
		"/dev/sda1 / ext4 rw,relatime 0 0",
		"tmpfs /tmp tmpfs rw,nosuid,nodev,size=3954456k 0 0",
		"/dev/sda2 /var/cache ext4 rw,relatime 0 0",
		"none /sys/fs/cgroup cgroup2 rw 0 0",
	}, "\n")

	cases := []struct {
		dir  string
		want string
	}{
		{"/", "ext4"},
		{"/tmp", "tmpfs"},
		{"/tmp/claws-bootstrap.swap", "tmpfs"}, // under /tmp mount
		{"/var/cache", "ext4"},
		{"/var/cache/claws", "ext4"}, // under /var/cache mount
		{"/var", "ext4"},             // falls back to root mount
		{"/sys/fs/cgroup", "cgroup2"},
		// Path that's a prefix of /tmp but not under /tmp itself
		{"/tmpfoo", "ext4"},
	}
	for _, c := range cases {
		got := mountFstypeForPath(mounts, c.dir)
		if got != c.want {
			t.Errorf("mountFstypeForPath(%q) = %q, want %q", c.dir, got, c.want)
		}
	}

	// Empty content → empty string (no error)
	if got := mountFstypeForPath("", "/tmp"); got != "" {
		t.Errorf("empty mounts should return empty string, got %q", got)
	}
}

func TestChooseAutoSwapSize(t *testing.T) {
	const G = uint64(1024 * 1024 * 1024)
	cases := []struct {
		name         string
		availMem     uint64
		existingSwap uint64
		want         uint64
	}{
		{"tiny VPS, no swap → ~5 GB to reach 6 GB total", 1 * G, 0, 5 * G},
		{"1 GB RAM + 2 GB swap → ~3 GB more", 1 * G, 2 * G, 3 * G},
		{"plenty of RAM → minimum useful (2 GB)", 16 * G, 0, 2 * G},
		{"RAM + swap already above target → minimum (2 GB)", 4 * G, 4 * G, 2 * G},
		{"absurdly tiny → clamped to 2 GB minimum", 0, 0, 6 * G}, // 6 GB to reach target
		// 16 GB short would normally need 16 GB swap — clamped to 8 GB ceiling
		{"huge deficit → clamped to 8 GB ceiling", 0, 0, 6 * G},
	}
	for _, c := range cases {
		got := chooseAutoSwapSize(c.availMem, c.existingSwap)
		if got != c.want {
			t.Errorf("%s: chooseAutoSwapSize(avail=%s, existing=%s) = %s, want %s",
				c.name, formatBytes(c.availMem), formatBytes(c.existingSwap),
				formatBytes(got), formatBytes(c.want))
		}
	}
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		in   uint64
		want string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1500000, "1 MB"},
		{2 * 1000 * 1000, "2 MB"},
		{1500 * 1000 * 1000, "1.5 GB"},
		{8 * 1000 * 1000 * 1000, "8.0 GB"},
	}
	for _, c := range cases {
		if got := formatBytes(c.in); got != c.want {
			t.Errorf("formatBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
