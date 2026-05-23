package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRoleCanRunCommand(t *testing.T) {
	admin := Role{Commands: []string{"*"}}
	if !roleCanRunCommand(&admin, "anything") {
		t.Error("admin should run any command")
	}

	operator := Role{Commands: []string{"start", "stop", "logs"}}
	if !roleCanRunCommand(&operator, "start") {
		t.Error("operator should run start")
	}
	if roleCanRunCommand(&operator, "remove") {
		t.Error("operator should not run remove")
	}
}

func TestRoleCanAccessInstance(t *testing.T) {
	unrestricted := Role{Instances: []string{}}
	if !roleCanAccessInstance(&unrestricted, "anything") {
		t.Error("unrestricted role should access any instance")
	}

	scoped := Role{Instances: []string{"team/alice"}}
	if !roleCanAccessInstance(&scoped, "team/alice") {
		t.Error("should access own instance")
	}
	if roleCanAccessInstance(&scoped, "team/bob") {
		t.Error("should not access other instance")
	}
}

func TestResolveRole(t *testing.T) {
	ac := &AccessConfig{
		Roles: map[string]Role{
			"admin":    {Users: []string{"root"}},
			"operator": {Users: []string{"deploy"}},
		},
	}

	name, role := resolveRole(ac, "root")
	if name != "admin" || role == nil {
		t.Error("should resolve root as admin")
	}

	name, role = resolveRole(ac, "deploy")
	if name != "operator" || role == nil {
		t.Error("should resolve deploy as operator")
	}

	name, role = resolveRole(ac, "unknown")
	if role != nil {
		t.Error("unknown user should have no role")
	}
}

func TestIntegration_AccessInit(t *testing.T) {
	root := t.TempDir()
	out, err := clawctl(t, root, "access", "init")
	if err != nil {
		t.Fatalf("access init failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Access control initialized") {
		t.Errorf("should confirm init: %s", out)
	}
	if _, err := os.Stat(filepath.Join(root, ".access.json")); err != nil {
		t.Error(".access.json not created")
	}
}

func TestIntegration_AccessGrant(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "access", "init")
	out, err := clawctl(t, root, "access", "grant", "bob", "operator")
	if err != nil {
		t.Fatalf("grant failed: %v\n%s", err, out)
	}

	out, _ = clawctl(t, root, "access", "show")
	if !strings.Contains(out, "bob") {
		t.Errorf("bob should be in access config: %s", out)
	}
}

func TestIntegration_AccessRevoke(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "access", "init")
	clawctl(t, root, "access", "grant", "bob", "operator")
	clawctl(t, root, "access", "revoke", "bob")

	out, _ := clawctl(t, root, "access", "show")
	if strings.Contains(out, "bob") {
		t.Errorf("bob should be removed: %s", out)
	}
}

func TestIntegration_TokenRotate(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "create", "alpha")

	envFile := filepath.Join(root, "alpha", "instance.env")
	oldToken := readEnvFromFile(t, envFile, "OPENCLAW_GATEWAY_TOKEN")

	out, err := clawctl(t, root, "token", "rotate", "alpha")
	if err != nil {
		t.Fatalf("token rotate failed: %v\n%s", err, out)
	}

	newToken := readEnvFromFile(t, envFile, "OPENCLAW_GATEWAY_TOKEN")
	if newToken == oldToken {
		t.Error("token should change after rotation")
	}
	if len(newToken) != 64 {
		t.Errorf("new token should be 64 hex chars, got %d", len(newToken))
	}
}

func TestIntegration_TokenShow(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "create", "alpha")

	out, err := clawctl(t, root, "token", "show", "alpha")
	if err != nil {
		t.Fatalf("token show failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "...") {
		t.Errorf("token show should truncate: %s", out)
	}
}

func TestIntegration_AuditLog(t *testing.T) {
	root := t.TempDir()

	// Enable audit via policy
	p := Policy{AuditLog: true}
	paths := Paths{Root: root, PortRegistry: filepath.Join(root, ".port-registry")}
	writePolicy(paths, p)

	clawctl(t, root, "create", "alpha")
	clawctl(t, root, "list")

	// Check audit log exists
	logPath := filepath.Join(root, ".audit.log")
	if _, err := os.Stat(logPath); err != nil {
		t.Error("audit log should exist when policy.auditLog is true")
	}

	data, _ := os.ReadFile(logPath)
	content := string(data)
	if !strings.Contains(content, "create") {
		t.Errorf("audit log should contain create command: %s", content)
	}
}
