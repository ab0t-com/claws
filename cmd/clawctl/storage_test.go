package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStorageConfig(t *testing.T) {
	paths := testPaths(t)

	// No config yet
	cfg := readStorageConfig(paths)
	if cfg.Configured {
		t.Error("should not be configured initially")
	}

	// Write and read back
	cfg = StorageConfig{
		S3Bucket:   "test-bucket",
		S3Region:   "ap-southeast-2",
		S3Prefix:   "openclaw/",
		MountPath:  "/mnt/s3/openclaw",
		Configured: true,
	}
	if err := writeStorageConfig(paths, cfg); err != nil {
		t.Fatal(err)
	}

	loaded := readStorageConfig(paths)
	if loaded.S3Bucket != "test-bucket" {
		t.Errorf("bucket: expected 'test-bucket', got '%s'", loaded.S3Bucket)
	}
	if !loaded.Configured {
		t.Error("should be configured after write")
	}
}

func TestIntegration_StorageStatus(t *testing.T) {
	root := t.TempDir()
	out, _ := clawctl(t, root, "storage", "status")
	if !strings.Contains(out, "Not configured") {
		t.Errorf("unconfigured storage should say so: %s", out)
	}
}

func TestIntegration_StorageSyncRequiresSetup(t *testing.T) {
	root := t.TempDir()
	_, err := clawctl(t, root, "storage", "sync")
	if err == nil {
		t.Error("sync without setup should fail")
	}
}

func TestIntegration_MigrateRequiresSetup(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "create", "alpha")
	_, err := clawctl(t, root, "migrate", "alpha", "--to", "s3")
	if err == nil {
		t.Error("migrate without storage setup should fail")
	}
}

func TestIntegration_StorageSetupRequiresBucket(t *testing.T) {
	root := t.TempDir()
	_, err := clawctl(t, root, "storage", "setup")
	if err == nil {
		t.Error("setup without --bucket should fail")
	}
}

func TestIntegration_StorageConfigPersists(t *testing.T) {
	root := t.TempDir()

	// Manually write storage config (since we can't call real AWS)
	cfg := StorageConfig{
		S3Bucket:   "test-bucket",
		S3Region:   "us-east-1",
		S3Prefix:   "openclaw/",
		MountPath:  filepath.Join(root, "mnt"),
		Configured: true,
	}
	paths := Paths{Root: root, PortRegistry: filepath.Join(root, ".port-registry")}
	writeStorageConfig(paths, cfg)

	out, _ := clawctl(t, root, "storage", "status")
	if !strings.Contains(out, "test-bucket") {
		t.Errorf("status should show configured bucket: %s", out)
	}
}

func TestIsMountedFalse(t *testing.T) {
	if isMounted("/nonexistent/path/that/does/not/exist") {
		t.Error("nonexistent path should not be mounted")
	}
}

func TestStorageConfigFileLocation(t *testing.T) {
	paths := testPaths(t)
	cfg := StorageConfig{S3Bucket: "test", Configured: true}
	writeStorageConfig(paths, cfg)

	expectedPath := filepath.Join(paths.Root, ".storage.json")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Error(".storage.json should be created in OPENCLAW_ROOT")
	}
}

func TestBuildSyncArgs_DefaultIsCopy(t *testing.T) {
	args := buildSyncArgs("/root", "bucket", "prefix/", false, false)
	if args[0] != "copy" {
		t.Errorf("default command should be 'copy', got '%s'", args[0])
	}
	// Should not contain "--dry-run"
	for _, a := range args {
		if a == "--dry-run" {
			t.Error("should not contain --dry-run when dryRun=false")
		}
	}
}

func TestBuildSyncArgs_MirrorIsSync(t *testing.T) {
	args := buildSyncArgs("/root", "bucket", "prefix/", true, false)
	if args[0] != "sync" {
		t.Errorf("mirror command should be 'sync', got '%s'", args[0])
	}
}

func TestBuildSyncArgs_DryRun(t *testing.T) {
	args := buildSyncArgs("/root", "bucket", "prefix/", false, true)
	found := false
	for _, a := range args {
		if a == "--dry-run" {
			found = true
		}
	}
	if !found {
		t.Error("should contain --dry-run")
	}
}

func TestBuildSyncArgs_ExcludesCredentials(t *testing.T) {
	args := buildSyncArgs("/root", "bucket", "prefix/", false, false)
	found := false
	for _, a := range args {
		if a == "*/credentials/**" {
			found = true
		}
	}
	if !found {
		t.Error("should exclude credentials")
	}
}
