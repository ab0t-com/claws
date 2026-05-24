package main

import (
	"strings"
	"testing"
)

func TestParseLegacyCrontab(t *testing.T) {
	body := `# claws-managed cron — agent: t/a
# Edit by editing the template profile and re-running ` + "`claws apply`" + `.

# DISABLED: skipped-one
# job: skipped-one
@hourly echo skip

# job: daily-summary
@daily echo daily

# job: hb
every 30m echo heartbeat
`
	jobs := parseLegacyCrontab(body)
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}
	byName := map[string]ProfileCronJob{}
	for _, j := range jobs {
		byName[j.Name] = j
	}
	if j := byName["daily-summary"]; j.Schedule != "@daily" || !strings.Contains(j.Command, "daily") {
		t.Errorf("daily-summary parse wrong: %+v", j)
	}
	if j := byName["hb"]; j.Schedule != "every 30m" {
		t.Errorf("hb schedule wrong: %+v", j)
	}
	if j := byName["skipped-one"]; j.Enabled == nil || *j.Enabled {
		t.Errorf("skipped-one should be disabled: %+v", j)
	}
}
