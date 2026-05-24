package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Task represents a unit of work dispatched from a manager to a worker.
type Task struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	CreatedBy   string `json:"created_by"`
	CreatedAt   string `json:"created_at"`
	Status      string `json:"status"` // pending, claimed, done
	ClaimedBy   string `json:"claimed_by,omitempty"`
	ClaimedAt   string `json:"claimed_at,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
	Result      string `json:"result,omitempty"`
}

// isFuseMount checks if the given path is on a FUSE filesystem (e.g., mountpoint-s3).
// Task queue operations (claim/complete) use os.Rename which is not supported on FUSE/S3 mounts.
func isFuseMount(path string) bool {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && strings.HasPrefix(path, fields[1]) {
			fsType := fields[2]
			if fsType == "fuse" || fsType == "fuse.s3fs" || fsType == "fuse.mountpoint-s3" || strings.HasPrefix(fsType, "fuse.") {
				return true
			}
		}
	}
	return false
}

func generateTaskID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func taskDir(paths Paths, groupName string) string {
	return filepath.Join(paths.Root, groupName, "shared", "tasks")
}

func readTask(path string) (Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Task{}, err
	}
	var t Task
	if err := json.Unmarshal(data, &t); err != nil {
		return Task{}, err
	}
	return t, nil
}

func writeTask(path string, t Task) error {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// findTask searches all three status dirs for a task by ID prefix.
func findTask(tasksDir, idPrefix string) (Task, string, string, error) {
	for _, status := range []string{"pending", "claimed", "done"} {
		dir := filepath.Join(tasksDir, status)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), idPrefix) && strings.HasSuffix(e.Name(), ".json") {
				path := filepath.Join(dir, e.Name())
				t, err := readTask(path)
				if err != nil {
					continue
				}
				return t, path, status, nil
			}
		}
	}
	return Task{}, "", "", fmt.Errorf("task '%s' not found", idPrefix)
}

// listTasks reads all tasks from all status dirs.
func listTasks(tasksDir string) []Task {
	var tasks []Task
	for _, status := range []string{"pending", "claimed", "done"} {
		dir := filepath.Join(tasksDir, status)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			t, err := readTask(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			t.Status = status
			tasks = append(tasks, t)
		}
	}
	return tasks
}

// ---------------------------------------------------------------------------
// claws task — task lifecycle management
// ---------------------------------------------------------------------------

func cmdTask(args []string) error {
	if len(args) < 1 {
		return errorf("usage: claws task <create|list|claim|complete|status> [args...]")
	}
	switch args[0] {
	case "create":
		return cmdTaskCreate(args[1:])
	case "list", "ls":
		return cmdTaskList(args[1:])
	case "claim":
		return cmdTaskClaim(args[1:])
	case "complete":
		return cmdTaskComplete(args[1:])
	case "status":
		return cmdTaskStatus(args[1:])
	default:
		return errorf("unknown task subcommand: %s", args[0])
	}
}

func cmdTaskCreate(args []string) error {
	if len(args) < 2 {
		return errorf("usage: claws task create <group> <title> [--from=<instance>] [--description=<text>]")
	}

	paths := resolvePaths()
	groupName := args[0]
	title := args[1]

	groupDir := filepath.Join(paths.Root, groupName)
	if !IsGroup(groupDir) {
		return errorf("group '%s' does not exist", groupName)
	}

	var from, description string
	for _, a := range args[2:] {
		switch {
		case strings.HasPrefix(a, "--from="):
			from = a[7:]
		case strings.HasPrefix(a, "--description="):
			description = a[14:]
		}
	}
	if from == "" {
		from = "cli"
	}

	td := taskDir(paths, groupName)
	pendingDir := filepath.Join(td, "pending")
	os.MkdirAll(pendingDir, 0755)

	task := Task{
		ID:          generateTaskID(),
		Title:       title,
		Description: description,
		CreatedBy:   from,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		Status:      "pending",
	}

	taskFile := filepath.Join(pendingDir, task.ID+".json")
	if err := writeTask(taskFile, task); err != nil {
		return err
	}

	info(fmt.Sprintf("Task created: %s", task.ID))
	fmt.Printf("  Group:   %s\n", groupName)
	fmt.Printf("  Title:   %s\n", task.Title)
	fmt.Printf("  Status:  pending\n")
	fmt.Printf("  File:    %s\n", taskFile)
	return nil
}

func cmdTaskList(args []string) error {
	if len(args) < 1 {
		return errorf("usage: claws task list <group> [--status=pending|claimed|done] [--json]")
	}

	paths := resolvePaths()
	groupName := args[0]

	groupDir := filepath.Join(paths.Root, groupName)
	if !IsGroup(groupDir) {
		return errorf("group '%s' does not exist", groupName)
	}

	filterStatus := flagValue(args[1:], "--status=")
	jsonMode := hasFlag(args, "--json")

	td := taskDir(paths, groupName)
	tasks := listTasks(td)

	if filterStatus != "" {
		var filtered []Task
		for _, t := range tasks {
			if t.Status == filterStatus {
				filtered = append(filtered, t)
			}
		}
		tasks = filtered
	}

	if jsonMode {
		if tasks == nil {
			fmt.Println("[]")
			return nil
		}
		data, _ := json.MarshalIndent(tasks, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found.")
		return nil
	}

	// Sort: pending first, then claimed, then done; within status by created_at
	statusOrder := map[string]int{"pending": 0, "claimed": 1, "done": 2}
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Status != tasks[j].Status {
			return statusOrder[tasks[i].Status] < statusOrder[tasks[j].Status]
		}
		return tasks[i].CreatedAt > tasks[j].CreatedAt
	})

	bold := "\033[1m"
	nc := "\033[0m"
	green := "\033[0;32m"
	yellow := "\033[0;33m"
	cyan := "\033[0;36m"

	fmt.Printf("%s%-18s %-10s %-15s %-40s%s\n", bold, "ID", "STATUS", "ASSIGNED", "TITLE", nc)
	fmt.Printf("%-18s %-10s %-15s %-40s\n", "──────────────────", "──────────", "───────────────", "────────────────────────────────────────")

	for _, t := range tasks {
		color := cyan
		switch t.Status {
		case "pending":
			color = yellow
		case "claimed":
			color = cyan
		case "done":
			color = green
		}
		assigned := t.ClaimedBy
		if assigned == "" {
			assigned = "—"
		}
		title := t.Title
		if len(title) > 40 {
			title = title[:37] + "..."
		}
		fmt.Printf("%-18s %s%-10s%s %-15s %-40s\n", t.ID, color, t.Status, nc, assigned, title)
	}
	return nil
}

func cmdTaskClaim(args []string) error {
	if len(args) < 3 {
		return errorf("usage: claws task claim <group> <task-id> --by=<instance>")
	}

	paths := resolvePaths()
	groupName := args[0]
	taskID := args[1]

	groupDir := filepath.Join(paths.Root, groupName)
	if !IsGroup(groupDir) {
		return errorf("group '%s' does not exist", groupName)
	}

	var claimedBy string
	for _, a := range args[2:] {
		if strings.HasPrefix(a, "--by=") {
			claimedBy = a[5:]
		}
	}
	if claimedBy == "" {
		return errorf("--by=<instance> is required")
	}

	td := taskDir(paths, groupName)

	// Warn if task dir is on a FUSE mount (os.Rename won't work)
	if isFuseMount(td) {
		return errorf("task queue is on a FUSE/S3 mount — os.Rename is not supported. Tasks only work on local storage.")
	}

	pendingDir := filepath.Join(td, "pending")
	claimedDir := filepath.Join(td, "claimed")
	os.MkdirAll(claimedDir, 0755)

	// Find the task in pending
	var taskPath string
	entries, err := os.ReadDir(pendingDir)
	if err != nil {
		return errorf("cannot read pending tasks: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), taskID) && strings.HasSuffix(e.Name(), ".json") {
			taskPath = filepath.Join(pendingDir, e.Name())
			break
		}
	}
	if taskPath == "" {
		return errorf("task '%s' not found in pending queue", taskID)
	}

	task, err := readTask(taskPath)
	if err != nil {
		return err
	}

	// Atomic move: pending → claimed
	destPath := filepath.Join(claimedDir, filepath.Base(taskPath))
	if err := os.Rename(taskPath, destPath); err != nil {
		return errorf("failed to claim task (may already be claimed): %v", err)
	}

	// Update task metadata
	task.Status = "claimed"
	task.ClaimedBy = claimedBy
	task.ClaimedAt = time.Now().UTC().Format(time.RFC3339)
	if err := writeTask(destPath, task); err != nil {
		return err
	}

	info(fmt.Sprintf("Task %s claimed by %s.", task.ID, claimedBy))
	return nil
}

func cmdTaskComplete(args []string) error {
	if len(args) < 2 {
		return errorf("usage: claws task complete <group> <task-id> [--result=<text>]")
	}

	paths := resolvePaths()
	groupName := args[0]
	taskID := args[1]

	groupDir := filepath.Join(paths.Root, groupName)
	if !IsGroup(groupDir) {
		return errorf("group '%s' does not exist", groupName)
	}

	var result string
	for _, a := range args[2:] {
		if strings.HasPrefix(a, "--result=") {
			result = a[9:]
		}
	}

	td := taskDir(paths, groupName)

	if isFuseMount(td) {
		return errorf("task queue is on a FUSE/S3 mount — os.Rename is not supported. Tasks only work on local storage.")
	}

	claimedDir := filepath.Join(td, "claimed")
	doneDir := filepath.Join(td, "done")
	os.MkdirAll(doneDir, 0755)

	// Find the task in claimed
	var taskPath string
	entries, err := os.ReadDir(claimedDir)
	if err != nil {
		return errorf("cannot read claimed tasks: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), taskID) && strings.HasSuffix(e.Name(), ".json") {
			taskPath = filepath.Join(claimedDir, e.Name())
			break
		}
	}
	if taskPath == "" {
		return errorf("task '%s' not found in claimed queue (must be claimed before completing)", taskID)
	}

	task, err := readTask(taskPath)
	if err != nil {
		return err
	}

	// Move: claimed → done
	destPath := filepath.Join(doneDir, filepath.Base(taskPath))
	if err := os.Rename(taskPath, destPath); err != nil {
		return errorf("failed to complete task: %v", err)
	}

	task.Status = "done"
	task.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	task.Result = result
	if err := writeTask(destPath, task); err != nil {
		return err
	}

	info(fmt.Sprintf("Task %s completed.", task.ID))
	return nil
}

func cmdTaskStatus(args []string) error {
	if len(args) < 2 {
		return errorf("usage: claws task status <group> <task-id>")
	}

	paths := resolvePaths()
	groupName := args[0]
	taskID := args[1]

	groupDir := filepath.Join(paths.Root, groupName)
	if !IsGroup(groupDir) {
		return errorf("group '%s' does not exist", groupName)
	}

	td := taskDir(paths, groupName)
	task, _, status, err := findTask(td, taskID)
	if err != nil {
		return err
	}
	task.Status = status

	bold := "\033[1m"
	nc := "\033[0m"

	fmt.Printf("%sTask: %s%s\n", bold, task.ID, nc)
	fmt.Printf("  Title:       %s\n", task.Title)
	if task.Description != "" {
		fmt.Printf("  Description: %s\n", task.Description)
	}
	fmt.Printf("  Status:      %s\n", task.Status)
	fmt.Printf("  Created by:  %s\n", task.CreatedBy)
	fmt.Printf("  Created at:  %s\n", task.CreatedAt)
	if task.ClaimedBy != "" {
		fmt.Printf("  Claimed by:  %s\n", task.ClaimedBy)
		fmt.Printf("  Claimed at:  %s\n", task.ClaimedAt)
	}
	if task.CompletedAt != "" {
		fmt.Printf("  Completed:   %s\n", task.CompletedAt)
	}
	if task.Result != "" {
		fmt.Printf("  Result:      %s\n", task.Result)
	}
	return nil
}
