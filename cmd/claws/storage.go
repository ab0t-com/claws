package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Storage configuration lives in ~/.openclaw/.storage.json
type StorageConfig struct {
	S3Bucket  string `json:"s3_bucket"`
	S3Region  string `json:"s3_region"`
	S3Prefix  string `json:"s3_prefix"` // key prefix inside bucket (default: "openclaw/")
	MountPath string `json:"mount_path"` // local FUSE mount point (default: /mnt/s3/openclaw)
	IAMRole   string `json:"iam_role"`   // instance role for S3 access
	Configured bool  `json:"configured"`
}

const storageConfigFile = ".storage.json"

func readStorageConfig(paths Paths) StorageConfig {
	data, err := os.ReadFile(filepath.Join(paths.Root, storageConfigFile))
	if err != nil {
		return StorageConfig{}
	}
	var cfg StorageConfig
	decodeStorageJSON(data, &cfg)
	return cfg
}

func writeStorageConfig(paths Paths, cfg StorageConfig) error {
	data := encodeStorageJSON(cfg)
	return os.WriteFile(filepath.Join(paths.Root, storageConfigFile), data, 0644)
}

// ---------------------------------------------------------------------------
// claws storage setup — configure S3 bucket + IAM + rclone
// ---------------------------------------------------------------------------

func cmdStorage(args []string) error {
	if len(args) < 1 {
		return errorf("usage: claws storage <setup|sync|mount|unmount|status>")
	}
	switch args[0] {
	case "setup":
		return cmdStorageSetup(args[1:])
	case "sync":
		return cmdStorageSync(args[1:])
	case "mount":
		return cmdStorageMount(args[1:])
	case "unmount":
		return cmdStorageUnmount(args[1:])
	case "status":
		return cmdStorageStatus(args[1:])
	case "cron":
		return cmdStorageCron(args[1:])
	default:
		return errorf("unknown storage subcommand: %s", args[0])
	}
}

func cmdStorageSetup(args []string) error {
	paths := resolvePaths()

	var bucket, region, profile string
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--bucket="):
			bucket = a[9:]
		case strings.HasPrefix(a, "--region="):
			region = a[9:]
		case strings.HasPrefix(a, "--profile="):
			profile = a[10:]
		}
	}

	if bucket == "" {
		return errorf("usage: claws storage setup --bucket=<name> [--region=ap-southeast-2] [--profile=ab0t]")
	}
	if region == "" {
		region = "ap-southeast-2"
	}

	info("Setting up S3 storage...")
	fmt.Printf("  Bucket:  %s\n", bucket)
	fmt.Printf("  Region:  %s\n", region)
	fmt.Println()

	// Step 1: Check AWS CLI
	if _, err := exec.LookPath("aws"); err != nil {
		return errorf("aws cli not found — install: https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html")
	}

	// Step 2: Create S3 bucket (idempotent)
	info("Creating S3 bucket (if not exists)...")
	createArgs := []string{"s3api", "create-bucket",
		"--bucket", bucket,
		"--region", region,
	}
	// ap-southeast-2 etc. need LocationConstraint
	if region != "us-east-1" {
		createArgs = append(createArgs, "--create-bucket-configuration", "LocationConstraint="+region)
	}
	if profile != "" {
		createArgs = append(createArgs, "--profile", profile)
	}
	cmd := exec.Command("aws", createArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		outStr := string(out)
		if strings.Contains(outStr, "BucketAlreadyOwnedByYou") || strings.Contains(outStr, "BucketAlreadyExists") {
			fmt.Println("  Bucket already exists — ok")
		} else {
			return errorf("failed to create bucket: %s", outStr)
		}
	} else {
		fmt.Println("  Bucket created")
	}

	// Step 3: Enable versioning (idempotent)
	info("Enabling bucket versioning...")
	versArgs := []string{"s3api", "put-bucket-versioning",
		"--bucket", bucket,
		"--region", region,
		"--versioning-configuration", "Status=Enabled",
	}
	if profile != "" {
		versArgs = append(versArgs, "--profile", profile)
	}
	cmd = exec.Command("aws", versArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		warn(fmt.Sprintf("could not enable versioning: %s", string(out)))
	} else {
		fmt.Println("  Versioning enabled")
	}

	// Step 4: Block public access (idempotent)
	info("Blocking public access...")
	blockArgs := []string{"s3api", "put-public-access-block",
		"--bucket", bucket,
		"--region", region,
		"--public-access-block-configuration",
		"BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true",
	}
	if profile != "" {
		blockArgs = append(blockArgs, "--profile", profile)
	}
	cmd = exec.Command("aws", blockArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		warn(fmt.Sprintf("could not block public access: %s", string(out)))
	} else {
		fmt.Println("  Public access blocked")
	}

	// Step 5: Check rclone
	rcloneInstalled := false
	if _, err := exec.LookPath("rclone"); err == nil {
		rcloneInstalled = true
		fmt.Println()
		info("rclone found — configuring remote...")

		// Create rclone remote config (idempotent)
		rcloneArgs := []string{"config", "create", "openclaw-s3", "s3",
			"provider", "AWS",
			"region", region,
			"bucket", bucket,
			"env_auth", "true", // use instance role or env credentials
		}
		cmd = exec.Command("rclone", rcloneArgs...)
		if out, err := cmd.CombinedOutput(); err != nil {
			warn(fmt.Sprintf("rclone config failed: %s", string(out)))
		} else {
			fmt.Println("  rclone remote 'openclaw-s3' configured")
		}
	} else {
		fmt.Println()
		warn("rclone not found — install for automatic backup:")
		fmt.Println("  curl https://rclone.org/install.sh | sudo bash")
	}

	// Step 6: Save config
	cfg := StorageConfig{
		S3Bucket:   bucket,
		S3Region:   region,
		S3Prefix:   "openclaw/",
		MountPath:  "/mnt/s3/openclaw",
		Configured: true,
	}
	if err := writeStorageConfig(paths, cfg); err != nil {
		return errorf("failed to save storage config: %v", err)
	}

	fmt.Println()
	info("Storage setup complete.")
	fmt.Println()
	fmt.Println("  Next steps:")
	if rcloneInstalled {
		fmt.Println("    claws storage sync                 # manual sync now")
		fmt.Println("    claws storage cron --enable        # auto-sync every hour")
	} else {
		fmt.Println("    Install rclone, then:")
		fmt.Println("    claws storage sync                 # manual sync")
	}
	fmt.Println("    claws storage mount                # FUSE mount (needs mountpoint-s3)")
	fmt.Println("    claws storage status               # check everything")
	return nil
}

// ---------------------------------------------------------------------------
// claws storage sync — run rclone sync to S3
// ---------------------------------------------------------------------------

func cmdStorageSync(args []string) error {
	paths := resolvePaths()
	cfg := readStorageConfig(paths)
	if !cfg.Configured {
		return errorf("storage not configured — run: claws storage setup --bucket=<name>")
	}

	if _, err := exec.LookPath("rclone"); err != nil {
		return errorf("rclone not found — install: curl https://rclone.org/install.sh | sudo bash")
	}

	dryRun := false
	mirror := false
	confirmed := false
	for _, a := range args {
		switch a {
		case "--dry-run":
			dryRun = true
		case "--mirror":
			mirror = true
		case "--yes":
			confirmed = true
		}
	}

	// Default: "copy" (additive only). --mirror uses "sync" (deletes destination-only files).
	rcloneCmd := "copy"
	if mirror {
		if !confirmed {
			warn("--mirror uses rclone sync which DELETES files on S3 that don't exist locally.")
			fmt.Print("  Continue? [y/N] ")
			var answer string
			fmt.Scanln(&answer)
			if answer != "y" && answer != "Y" {
				info("Aborted.")
				return nil
			}
		}
		rcloneCmd = "sync"
	}

	remote := fmt.Sprintf("openclaw-s3:%s/%s", cfg.S3Bucket, cfg.S3Prefix)
	info(fmt.Sprintf("Copying %s → %s (mode: %s)", paths.Root, remote, rcloneCmd))

	syncArgs := []string{rcloneCmd, paths.Root, remote,
		"--exclude", "*/credentials/**", // never sync credentials
		"--exclude", "*/.storage.json",  // meta file
		"--exclude", "*/sessions/**",    // session state is ephemeral
		"--exclude", "*/delivery-queue/**",
		"--exclude", "*/media/**",
		"-v",
	}
	if dryRun {
		syncArgs = append(syncArgs, "--dry-run")
		info("(dry run — no changes will be made)")
	}

	cmd := exec.Command("rclone", syncArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return errorf("rclone %s failed: %v", rcloneCmd, err)
	}

	info("Sync complete.")
	return nil
}

// buildSyncArgs builds the rclone command arguments for testing/inspection.
func buildSyncArgs(root, bucket, prefix string, mirror, dryRun bool) []string {
	rcloneCmd := "copy"
	if mirror {
		rcloneCmd = "sync"
	}
	remote := fmt.Sprintf("openclaw-s3:%s/%s", bucket, prefix)
	args := []string{rcloneCmd, root, remote,
		"--exclude", "*/credentials/**",
		"--exclude", "*/.storage.json",
		"--exclude", "*/sessions/**",
		"--exclude", "*/delivery-queue/**",
		"--exclude", "*/media/**",
		"-v",
	}
	if dryRun {
		args = append(args, "--dry-run")
	}
	return args
}

// ---------------------------------------------------------------------------
// claws storage cron — enable/disable periodic sync
// ---------------------------------------------------------------------------

const cronComment = "# claws-storage-sync"

func cmdStorageCron(args []string) error {
	paths := resolvePaths()
	cfg := readStorageConfig(paths)
	if !cfg.Configured {
		return errorf("storage not configured — run: claws storage setup --bucket=<name>")
	}

	enable := false
	disable := false
	interval := "hourly"
	for _, a := range args {
		switch {
		case a == "--enable":
			enable = true
		case a == "--disable":
			disable = true
		case strings.HasPrefix(a, "--interval="):
			interval = a[11:]
		}
	}

	if !enable && !disable {
		return errorf("usage: claws storage cron --enable|--disable [--interval=hourly]")
	}

	// Find the claws binary path
	exe, _ := os.Executable()

	if disable {
		// Remove our cron entry
		removeCronEntry()
		info("Storage sync cron disabled.")
		return nil
	}

	// Build cron schedule
	var schedule string
	switch interval {
	case "hourly":
		schedule = "0 * * * *"
	case "daily":
		schedule = "0 3 * * *" // 3am UTC
	case "15m":
		schedule = "*/15 * * * *"
	case "30m":
		schedule = "*/30 * * * *"
	default:
		schedule = interval // allow raw cron expression
	}

	cronLine := fmt.Sprintf("%s OPENCLAW_ROOT=%s %s storage sync >> /var/log/claws-sync.log 2>&1 %s",
		schedule, paths.Root, exe, cronComment)

	// Remove old entry first, then add new
	removeCronEntry()
	addCronEntry(cronLine)

	info(fmt.Sprintf("Storage sync cron enabled (%s).", interval))
	fmt.Printf("  Schedule: %s\n", schedule)
	fmt.Printf("  Log:      /var/log/claws-sync.log\n")
	return nil
}

func removeCronEntry() {
	cmd := exec.Command("crontab", "-l")
	out, err := cmd.Output()
	if err != nil {
		return
	}
	var lines []string
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, cronComment) {
			lines = append(lines, line)
		}
	}
	newCrontab := strings.Join(lines, "\n")
	cmd = exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(newCrontab)
	cmd.Run()
}

func addCronEntry(line string) {
	cmd := exec.Command("crontab", "-l")
	existing, _ := cmd.Output()
	newCrontab := string(existing) + line + "\n"
	cmd = exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(newCrontab)
	cmd.Run()
}

// ---------------------------------------------------------------------------
// claws storage mount — FUSE mount shared workspace from S3
// ---------------------------------------------------------------------------

func cmdStorageMount(args []string) error {
	paths := resolvePaths()
	cfg := readStorageConfig(paths)
	if !cfg.Configured {
		return errorf("storage not configured — run: claws storage setup --bucket=<name>")
	}

	if _, err := exec.LookPath("mount-s3"); err != nil {
		return errorf("mountpoint-s3 not found — install: https://github.com/awslabs/mountpoint-s3")
	}

	mountPath := cfg.MountPath
	for _, a := range args {
		if strings.HasPrefix(a, "--path=") {
			mountPath = a[7:]
		}
	}

	// Check if already mounted
	if isMounted(mountPath) {
		info(fmt.Sprintf("Already mounted at %s", mountPath))
		return nil
	}

	// Create mount point
	os.MkdirAll(mountPath, 0755)

	info(fmt.Sprintf("Mounting s3://%s/%s at %s", cfg.S3Bucket, cfg.S3Prefix, mountPath))

	cmd := exec.Command("mount-s3",
		"--prefix", cfg.S3Prefix,
		"--allow-other",
		"--region", cfg.S3Region,
		cfg.S3Bucket,
		mountPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errorf("mount failed: %v — ensure IAM role has s3:GetObject/PutObject/ListBucket", err)
	}

	info(fmt.Sprintf("Mounted at %s", mountPath))
	fmt.Println()
	fmt.Println("  To use with shared workspace:")
	fmt.Printf("    ln -sf %s/shared/workspace ~/.openclaw/shared/workspace\n", mountPath)
	fmt.Println()
	fmt.Println("  To persist across reboots, add to /etc/fstab:")
	fmt.Printf("    # mount-s3 %s %s fuse _netdev,allow_other,prefix=%s,region=%s 0 0\n",
		cfg.S3Bucket, mountPath, cfg.S3Prefix, cfg.S3Region)
	return nil
}

func cmdStorageUnmount(args []string) error {
	paths := resolvePaths()
	cfg := readStorageConfig(paths)
	mountPath := cfg.MountPath
	for _, a := range args {
		if strings.HasPrefix(a, "--path=") {
			mountPath = a[7:]
		}
	}

	if !isMounted(mountPath) {
		info(fmt.Sprintf("Not mounted at %s", mountPath))
		return nil
	}

	cmd := exec.Command("umount", mountPath)
	if err := cmd.Run(); err != nil {
		// Try fusermount
		cmd = exec.Command("fusermount", "-u", mountPath)
		if err := cmd.Run(); err != nil {
			return errorf("unmount failed: %v", err)
		}
	}

	info(fmt.Sprintf("Unmounted %s", mountPath))
	return nil
}

func isMounted(path string) bool {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}
	return strings.Contains(string(data), path)
}

// ---------------------------------------------------------------------------
// claws storage status — show storage config and state
// ---------------------------------------------------------------------------

func cmdStorageStatus(args []string) error {
	paths := resolvePaths()
	cfg := readStorageConfig(paths)

	bold := "\033[1m"
	nc := "\033[0m"
	green := "\033[0;32m"
	red := "\033[0;31m"

	fmt.Printf("%sStorage Configuration%s\n\n", bold, nc)

	if !cfg.Configured {
		fmt.Println("  Not configured. Run: claws storage setup --bucket=<name>")
		return nil
	}

	fmt.Printf("  S3 Bucket:     s3://%s/%s\n", cfg.S3Bucket, cfg.S3Prefix)
	fmt.Printf("  Region:        %s\n", cfg.S3Region)
	fmt.Printf("  Mount Path:    %s\n", cfg.MountPath)
	fmt.Println()

	// Check tools
	fmt.Printf("%sTools%s\n", bold, nc)
	checkTool := func(name, installHint string) {
		if _, err := exec.LookPath(name); err == nil {
			fmt.Printf("  %s%-12s installed%s\n", green, name, nc)
		} else {
			fmt.Printf("  %s%-12s missing%s  — %s\n", red, name, nc, installHint)
		}
	}
	checkTool("aws", "https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html")
	checkTool("rclone", "curl https://rclone.org/install.sh | sudo bash")
	checkTool("mount-s3", "https://github.com/awslabs/mountpoint-s3")
	fmt.Println()

	// Check mount
	fmt.Printf("%sMount%s\n", bold, nc)
	if isMounted(cfg.MountPath) {
		fmt.Printf("  %s%s  mounted%s\n", green, cfg.MountPath, nc)
	} else {
		fmt.Printf("  %s%s  not mounted%s\n", red, cfg.MountPath, nc)
	}
	fmt.Println()

	// Check cron
	fmt.Printf("%sCron%s\n", bold, nc)
	cmd := exec.Command("crontab", "-l")
	if out, err := cmd.Output(); err == nil && strings.Contains(string(out), cronComment) {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, cronComment) {
				fmt.Printf("  %senabled%s: %s\n", green, nc, strings.TrimSuffix(line, " "+cronComment))
				break
			}
		}
	} else {
		fmt.Printf("  %sdisabled%s — run: claws storage cron --enable\n", red, nc)
	}

	// Check last sync time
	fmt.Println()
	fmt.Printf("%sLast Sync%s\n", bold, nc)
	logFile := "/var/log/claws-sync.log"
	if fi, err := os.Stat(logFile); err == nil {
		fmt.Printf("  %s (modified %s)\n", logFile, fi.ModTime().Format(time.RFC3339))
	} else {
		fmt.Println("  No sync log found")
	}

	return nil
}

// ---------------------------------------------------------------------------
// claws migrate --to s3 — move workspace to S3 mount
// ---------------------------------------------------------------------------

func cmdMigrate(args []string) error {
	// v1.6+: data migrations (cron, uuids, all) routed via cmdMigrateData.
	// Storage migration retains its original `claws migrate <instance> --to s3` form.
	if isDataMigration(args) {
		return cmdMigrateData(args)
	}
	if len(args) < 1 {
		return errorf("usage: claws migrate <name> --to s3 [--cleanup]   OR   claws migrate <cron|uuids|all>")
	}
	paths := resolvePaths()
	name := args[0]
	if err := requireInstance(paths, name); err != nil {
		return err
	}

	var target string
	cleanup := false
	for _, a := range args[1:] {
		switch {
		case strings.HasPrefix(a, "--to="):
			target = a[5:]
		case a == "--to":
			// next arg
		case a == "s3":
			target = "s3"
		case a == "--cleanup":
			cleanup = true
		}
	}

	if target != "s3" {
		return errorf("usage: claws migrate <name> --to s3 [--cleanup]")
	}

	cfg := readStorageConfig(paths)
	if !cfg.Configured {
		return errorf("storage not configured — run: claws storage setup --bucket=<name>")
	}
	if !isMounted(cfg.MountPath) {
		return errorf("S3 not mounted at %s — run: claws storage mount", cfg.MountPath)
	}

	ref, _ := ParseRef(name)
	dir := ref.Dir(paths)
	localWorkspace := filepath.Join(dir, "workspace")
	s3Workspace := filepath.Join(cfg.MountPath, name, "workspace")

	info(fmt.Sprintf("Migrating '%s' workspace to S3...", name))
	fmt.Printf("  From: %s\n", localWorkspace)
	fmt.Printf("  To:   %s\n", s3Workspace)
	fmt.Println()

	// Step 1: Stop instance
	info("Stopping instance...")
	dcRun(paths, ref.RegistryName(), "stop")

	// Step 2: Copy workspace to S3 mount
	info("Copying workspace...")
	os.MkdirAll(s3Workspace, 0755)
	cmd := exec.Command("rsync", "-a", "--progress", localWorkspace+"/", s3Workspace+"/")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errorf("rsync failed: %v — instance is stopped, restart manually", err)
	}

	// Step 3: Update instance.env to point to S3 workspace
	envFile := filepath.Join(dir, "instance.env")
	updateEnvValue(envFile, "OPENCLAW_WORKSPACE_DIR", s3Workspace)

	// Step 4: Restart
	info("Starting instance with S3 workspace...")
	dcRun(paths, ref.RegistryName(), "up", "-d", gatewayService(paths, ref.RegistryName()))

	if cleanup {
		info("Removing local workspace copy...")
		os.RemoveAll(localWorkspace)
		os.MkdirAll(localWorkspace, 0755) // keep empty dir so paths don't break
	}

	info(fmt.Sprintf("Migration complete. '%s' workspace is now on S3.", name))
	if !cleanup {
		fmt.Printf("  Local copy still at %s — use --cleanup to remove\n", localWorkspace)
	}
	return nil
}

func decodeStorageJSON(data []byte, v any) {
	json.Unmarshal(data, v)
}

func encodeStorageJSON(v any) []byte {
	data, _ := json.MarshalIndent(v, "", "  ")
	return append(data, '\n')
}
