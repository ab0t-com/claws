package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// clawctl image — manage Docker images
// ---------------------------------------------------------------------------

func cmdImage(args []string) error {
	if len(args) < 1 {
		return errorf("usage: clawctl image <list|pull|pin>")
	}
	switch args[0] {
	case "list", "ls":
		return cmdImageList(args[1:])
	case "pull":
		return cmdImagePull(args[1:])
	case "pin":
		return cmdImagePin(args[1:])
	default:
		return errorf("unknown image subcommand: %s", args[0])
	}
}

func cmdImageList(args []string) error {
	cmd := exec.Command("docker", "images", "--format", "table {{.Repository}}:{{.Tag}}\t{{.Size}}\t{{.CreatedSince}}", "--filter", "reference=openclaw*")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// Fallback: show all images
		cmd = exec.Command("docker", "images", "--format", "table {{.Repository}}:{{.Tag}}\t{{.Size}}\t{{.CreatedSince}}")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	return nil
}

func cmdImagePull(args []string) error {
	if len(args) < 1 {
		return errorf("usage: clawctl image pull <image:tag>")
	}
	image := args[0]

	// Policy check
	paths := resolvePaths()
	policy := readPolicy(paths)
	if err := policy.enforceImagePolicy(image); err != nil {
		return err
	}

	info(fmt.Sprintf("Pulling %s...", image))
	cmd := exec.Command("docker", "pull", image)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errorf("pull failed: %v", err)
	}
	info(fmt.Sprintf("Image %s pulled.", image))
	return nil
}

func cmdImagePin(args []string) error {
	if len(args) < 2 {
		return errorf("usage: clawctl image pin <instance> <image:tag>")
	}
	paths := resolvePaths()
	name := args[0]
	image := args[1]

	if err := requireInstance(paths, name); err != nil {
		return err
	}

	// Policy check
	policy := readPolicy(paths)
	if err := policy.enforceImagePolicy(image); err != nil {
		return err
	}

	// Verify image exists
	if err := exec.Command("docker", "image", "inspect", image).Run(); err != nil {
		return errorf("image '%s' not found locally — pull it first: clawctl image pull %s", image, image)
	}

	ref, _ := ParseRef(name)
	envFile := filepath.Join(ref.Dir(paths), "instance.env")
	updateEnvValue(envFile, "OPENCLAW_IMAGE", image)

	info(fmt.Sprintf("Pinned %s to image %s", name, image))
	fmt.Printf("  Restart to apply: clawctl restart %s --hard\n", name)
	return nil
}

// ---------------------------------------------------------------------------
// clawctl upgrade — upgrade instance image with health-check rollback
// ---------------------------------------------------------------------------

func cmdUpgrade(args []string) error {
	if len(args) < 1 {
		return errorf("usage: clawctl upgrade <instance|--all> [--image=<image:tag>]")
	}

	paths := resolvePaths()
	all := hasFlag(args, "--all")

	var targetImage string
	for _, a := range args {
		if strings.HasPrefix(a, "--image=") {
			targetImage = a[8:]
		}
	}

	var names []string
	if all {
		entries, err := readRegistry(paths)
		if err != nil {
			return err
		}
		for _, e := range entries {
			names = append(names, e.Name)
		}
	} else {
		name := args[0]
		if strings.HasPrefix(name, "--") {
			return errorf("usage: clawctl upgrade <instance|--all> [--image=<image:tag>]")
		}
		if err := requireInstance(paths, name); err != nil {
			return err
		}
		names = []string{name}
	}

	// Policy check on target image
	if targetImage != "" {
		policy := readPolicy(paths)
		if err := policy.enforceImagePolicy(targetImage); err != nil {
			return err
		}
		// Verify image exists
		if err := exec.Command("docker", "image", "inspect", targetImage).Run(); err != nil {
			return errorf("image '%s' not found — pull it first: clawctl image pull %s", targetImage, targetImage)
		}
	}

	var failed []string
	for _, name := range names {
		if err := upgradeInstance(paths, name, targetImage); err != nil {
			warn(fmt.Sprintf("upgrade failed for '%s': %v", name, err))
			failed = append(failed, name)
		}
	}

	if len(failed) > 0 {
		return errorf("%d instance(s) failed to upgrade: %s", len(failed), strings.Join(failed, ", "))
	}
	return nil
}

func upgradeInstance(paths Paths, name, targetImage string) error {
	ref, _ := ParseRef(name)
	dir := ref.Dir(paths)
	envFile := filepath.Join(dir, "instance.env")

	// Save current image for rollback
	previousImage := readEnvValue(envFile, "OPENCLAW_IMAGE")
	if previousImage == "" {
		previousImage = "openclaw:local"
	}

	newImage := targetImage
	if newImage == "" {
		newImage = previousImage // just recreate with same image (picks up compose changes)
	}

	if newImage == previousImage && targetImage == "" {
		info(fmt.Sprintf("%s — already on %s, recreating container...", name, previousImage))
	} else {
		info(fmt.Sprintf("%s — upgrading %s → %s", name, previousImage, newImage))
		updateEnvValue(envFile, "OPENCLAW_IMAGE", newImage)
	}

	// Stop old container
	dcRun(paths, name, "down")

	// Start with new image
	if err := dcRun(paths, name, "up", "-d", gatewayService(paths, name)); err != nil {
		// Rollback
		warn(fmt.Sprintf("start failed — rolling back to %s", previousImage))
		updateEnvValue(envFile, "OPENCLAW_IMAGE", previousImage)
		dcRun(paths, name, "up", "-d", gatewayService(paths, name))
		return errorf("upgrade failed, rolled back to %s", previousImage)
	}

	// Wait for health
	port := readEnvValue(envFile, "OPENCLAW_GATEWAY_PORT")
	healthy := false
	for i := 0; i < 15; i++ {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%s%s", port, mustResolveRuntime(paths, name).HealthEndpoint))
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			healthy = true
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(2 * time.Second)
	}

	if !healthy {
		// Rollback
		warn(fmt.Sprintf("%s — health check failed after 30s — rolling back to %s", name, previousImage))
		dcRun(paths, name, "down")
		updateEnvValue(envFile, "OPENCLAW_IMAGE", previousImage)
		dcRun(paths, name, "up", "-d", gatewayService(paths, name))
		return errorf("health check failed, rolled back to %s", previousImage)
	}

	info(fmt.Sprintf("%s — upgraded and healthy on %s", name, newImage))
	return nil
}
