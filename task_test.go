package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupGroupForTasks(t *testing.T, root string) {
	t.Helper()
	clawctl(t, root, "group", "create", "team")
	clawctl(t, root, "create", "team/lead", "--role=manager")
	clawctl(t, root, "create", "team/dev1", "--role=worker", "--manager=lead")
}

func TestIntegration_TaskCreate(t *testing.T) {
	root := t.TempDir()
	setupGroupForTasks(t, root)

	out, err := clawctl(t, root, "task", "create", "team", "summarize-report", "--from=lead")
	if err != nil {
		t.Fatalf("task create failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Task created") {
		t.Errorf("should confirm creation: %s", out)
	}
	if !strings.Contains(out, "pending") {
		t.Errorf("should show pending status: %s", out)
	}

	// Verify file exists in pending/
	pendingDir := filepath.Join(root, "team", "shared", "tasks", "pending")
	entries, err := os.ReadDir(pendingDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 pending task, got %d", len(entries))
	}
	if !strings.HasSuffix(entries[0].Name(), ".json") {
		t.Error("task file should be .json")
	}

	// Verify JSON content
	task, err := readTask(filepath.Join(pendingDir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if task.Title != "summarize-report" {
		t.Errorf("title should be 'summarize-report', got '%s'", task.Title)
	}
	if task.CreatedBy != "lead" {
		t.Errorf("created_by should be 'lead', got '%s'", task.CreatedBy)
	}
}

func TestIntegration_TaskClaim(t *testing.T) {
	root := t.TempDir()
	setupGroupForTasks(t, root)

	out, _ := clawctl(t, root, "task", "create", "team", "do-thing", "--from=lead")
	taskID := extractTaskID(t, out)

	out, err := clawctl(t, root, "task", "claim", "team", taskID, "--by=dev1")
	if err != nil {
		t.Fatalf("task claim failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "claimed by dev1") {
		t.Errorf("should confirm claim: %s", out)
	}

	// Verify moved from pending to claimed
	pendingEntries, _ := os.ReadDir(filepath.Join(root, "team", "shared", "tasks", "pending"))
	if len(pendingEntries) != 0 {
		t.Error("pending dir should be empty after claim")
	}
	claimedEntries, _ := os.ReadDir(filepath.Join(root, "team", "shared", "tasks", "claimed"))
	if len(claimedEntries) != 1 {
		t.Fatalf("claimed dir should have 1 entry, got %d", len(claimedEntries))
	}

	task, _ := readTask(filepath.Join(root, "team", "shared", "tasks", "claimed", claimedEntries[0].Name()))
	if task.ClaimedBy != "dev1" {
		t.Errorf("claimed_by should be 'dev1', got '%s'", task.ClaimedBy)
	}
}

func TestIntegration_TaskComplete(t *testing.T) {
	root := t.TempDir()
	setupGroupForTasks(t, root)

	out, _ := clawctl(t, root, "task", "create", "team", "build-feature", "--from=lead")
	taskID := extractTaskID(t, out)
	clawctl(t, root, "task", "claim", "team", taskID, "--by=dev1")

	out, err := clawctl(t, root, "task", "complete", "team", taskID, "--result=done-and-tested")
	if err != nil {
		t.Fatalf("task complete failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "completed") {
		t.Errorf("should confirm completion: %s", out)
	}

	// Verify moved to done
	doneEntries, _ := os.ReadDir(filepath.Join(root, "team", "shared", "tasks", "done"))
	if len(doneEntries) != 1 {
		t.Fatalf("done dir should have 1 entry, got %d", len(doneEntries))
	}

	task, _ := readTask(filepath.Join(root, "team", "shared", "tasks", "done", doneEntries[0].Name()))
	if task.Result != "done-and-tested" {
		t.Errorf("result should be 'done-and-tested', got '%s'", task.Result)
	}
	if task.CompletedAt == "" {
		t.Error("completed_at should be set")
	}
}

func TestIntegration_TaskList(t *testing.T) {
	root := t.TempDir()
	setupGroupForTasks(t, root)

	clawctl(t, root, "task", "create", "team", "task-one", "--from=lead")
	out2, _ := clawctl(t, root, "task", "create", "team", "task-two", "--from=lead")
	taskID2 := extractTaskID(t, out2)
	clawctl(t, root, "task", "claim", "team", taskID2, "--by=dev1")

	out, err := clawctl(t, root, "task", "list", "team")
	if err != nil {
		t.Fatalf("task list failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "task-one") {
		t.Error("list should show task-one")
	}
	if !strings.Contains(out, "task-two") {
		t.Error("list should show task-two")
	}
	if !strings.Contains(out, "pending") {
		t.Error("list should show pending status")
	}
	if !strings.Contains(out, "claimed") {
		t.Error("list should show claimed status")
	}
}

func TestIntegration_TaskListFilterStatus(t *testing.T) {
	root := t.TempDir()
	setupGroupForTasks(t, root)

	clawctl(t, root, "task", "create", "team", "pending-task", "--from=lead")
	out2, _ := clawctl(t, root, "task", "create", "team", "claimed-task", "--from=lead")
	taskID2 := extractTaskID(t, out2)
	clawctl(t, root, "task", "claim", "team", taskID2, "--by=dev1")

	out, _ := clawctl(t, root, "task", "list", "team", "--status=pending")
	if !strings.Contains(out, "pending-task") {
		t.Error("pending filter should show pending task")
	}
	if strings.Contains(out, "claimed-task") {
		t.Error("pending filter should not show claimed task")
	}
}

func TestIntegration_TaskClaimNonexistent(t *testing.T) {
	root := t.TempDir()
	setupGroupForTasks(t, root)

	_, err := clawctl(t, root, "task", "claim", "team", "nonexistent-id", "--by=dev1")
	if err == nil {
		t.Error("claiming nonexistent task should fail")
	}
}

func TestIntegration_TaskDoubleClaimFails(t *testing.T) {
	root := t.TempDir()
	setupGroupForTasks(t, root)

	out, _ := clawctl(t, root, "task", "create", "team", "contested-task", "--from=lead")
	taskID := extractTaskID(t, out)

	// First claim succeeds
	_, err := clawctl(t, root, "task", "claim", "team", taskID, "--by=dev1")
	if err != nil {
		t.Fatalf("first claim should succeed: %v", err)
	}

	// Second claim fails (task no longer in pending)
	_, err = clawctl(t, root, "task", "claim", "team", taskID, "--by=dev1")
	if err == nil {
		t.Error("second claim should fail — task already claimed")
	}
}

func TestIntegration_TaskStatus(t *testing.T) {
	root := t.TempDir()
	setupGroupForTasks(t, root)

	out, _ := clawctl(t, root, "task", "create", "team", "status-check", "--from=lead", "--description=test-desc")
	taskID := extractTaskID(t, out)

	out, err := clawctl(t, root, "task", "status", "team", taskID)
	if err != nil {
		t.Fatalf("task status failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, taskID) {
		t.Error("status should show task ID")
	}
	if !strings.Contains(out, "status-check") {
		t.Error("status should show title")
	}
	if !strings.Contains(out, "test-desc") {
		t.Error("status should show description")
	}
}

func TestIntegration_TaskLifecycle(t *testing.T) {
	root := t.TempDir()
	setupGroupForTasks(t, root)

	// Create
	out, _ := clawctl(t, root, "task", "create", "team", "full-lifecycle", "--from=lead")
	taskID := extractTaskID(t, out)

	// List — should be pending
	out, _ = clawctl(t, root, "task", "list", "team")
	if !strings.Contains(out, "pending") {
		t.Error("should be pending")
	}

	// Claim
	clawctl(t, root, "task", "claim", "team", taskID, "--by=dev1")

	// Complete
	clawctl(t, root, "task", "complete", "team", taskID, "--result=all-done")

	// Status — should be done
	out, _ = clawctl(t, root, "task", "status", "team", taskID)
	if !strings.Contains(out, "all-done") {
		t.Error("should show result")
	}

	// List done
	out, _ = clawctl(t, root, "task", "list", "team", "--status=done")
	if !strings.Contains(out, "full-lifecycle") {
		t.Error("done filter should show completed task")
	}
}

func TestIntegration_TaskHelp(t *testing.T) {
	root := t.TempDir()
	out, _ := clawctl(t, root, "help")
	if !strings.Contains(out, "Tasks") || !strings.Contains(out, "task create") {
		t.Errorf("help should include Tasks section: %s", out)
	}
}

// extractTaskID parses the task ID from "Task created: <id>" output.
func extractTaskID(t *testing.T, output string) string {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "Task created:") {
			parts := strings.Split(line, "Task created: ")
			if len(parts) == 2 {
				// Strip ANSI codes
				id := strings.TrimSpace(parts[1])
				id = stripAnsi(id)
				return id
			}
		}
	}
	t.Fatalf("could not extract task ID from output:\n%s", output)
	return ""
}

func stripAnsi(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\033' {
			// Skip until 'm'
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++ // skip 'm'
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
}
