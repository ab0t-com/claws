package main

import "testing"

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
