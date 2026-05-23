package main

import (
	"strings"
	"testing"
)

func TestGenerateProxyConfig_PathMode(t *testing.T) {
	instances := []proxyInstance{
		{Name: "alpha", Port: "18789", Token: "tok-a"},
		{Name: "bravo", Port: "18889", Token: "tok-b"},
	}

	config := generateProxyConfig("example.com", "path", instances, false)

	if !strings.Contains(config, "example.com {") {
		t.Error("should contain domain block")
	}
	if !strings.Contains(config, "handle_path /alpha/*") {
		t.Error("should contain alpha path")
	}
	if !strings.Contains(config, "handle_path /bravo/*") {
		t.Error("should contain bravo path")
	}
	if !strings.Contains(config, "reverse_proxy 127.0.0.1:18789") {
		t.Error("should contain alpha port")
	}
	if !strings.Contains(config, "reverse_proxy 127.0.0.1:18889") {
		t.Error("should contain bravo port")
	}
}

func TestGenerateProxyConfig_SubdomainMode(t *testing.T) {
	instances := []proxyInstance{
		{Name: "alpha", Port: "18789"},
		{Name: "sarah", Group: "backend", Port: "18889"},
	}

	config := generateProxyConfig("example.com", "subdomain", instances, false)

	if !strings.Contains(config, "alpha.example.com {") {
		t.Error("should contain alpha subdomain")
	}
	if !strings.Contains(config, "sarah-backend.example.com {") {
		t.Error("should contain grouped subdomain")
	}
}

func TestGenerateProxyConfig_WithAuth(t *testing.T) {
	instances := []proxyInstance{
		{Name: "alpha", Port: "18789", Token: "secret-token-123"},
	}

	config := generateProxyConfig("example.com", "path", instances, true)

	if !strings.Contains(config, "header_up Authorization") {
		t.Error("auth mode should include Authorization header")
	}
	if !strings.Contains(config, "Bearer secret-token-123") {
		t.Error("should contain the instance token")
	}
}

func TestGenerateProxyConfig_NoAuth(t *testing.T) {
	instances := []proxyInstance{
		{Name: "alpha", Port: "18789", Token: "secret-token-123"},
	}

	config := generateProxyConfig("example.com", "path", instances, false)

	if strings.Contains(config, "header_up Authorization") {
		t.Error("no-auth mode should not include Authorization header")
	}
}

func TestGenerateProxyConfig_EmptyPort(t *testing.T) {
	instances := []proxyInstance{
		{Name: "alpha", Port: ""},
		{Name: "bravo", Port: "18889"},
	}

	config := generateProxyConfig("example.com", "path", instances, false)

	if strings.Contains(config, "alpha") {
		t.Error("should skip instances with empty port")
	}
	if !strings.Contains(config, "bravo") {
		t.Error("should include instances with port")
	}
}

func TestGenerateProxyConfig_GroupedPath(t *testing.T) {
	instances := []proxyInstance{
		{Name: "sarah", Group: "backend", Port: "18789"},
	}

	config := generateProxyConfig("example.com", "path", instances, false)

	if !strings.Contains(config, "handle_path /backend/sarah/*") {
		t.Error("grouped instance should have group/name path")
	}
}

func TestIntegration_ProxySetupDryRun(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "create", "alpha")

	out, err := clawctl(t, root, "proxy", "setup", "--domain=test.com", "--dry-run")
	if err != nil {
		t.Fatalf("proxy setup --dry-run failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "dry run") {
		t.Error("should indicate dry run")
	}
	if !strings.Contains(out, "test.com") {
		t.Error("should contain domain in output")
	}
}
