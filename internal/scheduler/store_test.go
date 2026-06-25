package scheduler

import (
	"testing"
	"time"
)

func TestParseCron(t *testing.T) {
	tests := []struct {
		expr   string
		valid  bool
	}{
		{"*/5 * * * *", true},
		{"0 2 * * *", true},
		{"0 0 1 * *", true},
		{"0 0 * * 0", true},
		{"30 3 15 6 *", true},
		{"0,30 * * * *", true},
		{"0-30 * * * *", true},
		{"*/15 8-17 * * 1-5", true},
		{"* * * *", false},      // too few fields
		{"* * * * * *", false},  // too many fields
		{"60 * * * *", false},   // invalid minute
		{"* 25 * * *", false},   // invalid hour
		{"* * 0 * *", false},    // invalid DOM
		{"* * * 13 *", false},   // invalid month
		{"* * * * 7", false},    // invalid DOW
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			err := validateCron(tt.expr)
			if (err == nil) != tt.valid {
				t.Errorf("validateCron(%q) = %v, want valid=%v", tt.expr, err, tt.valid)
			}
		})
	}
}

func TestNextCron(t *testing.T) {
	// Use UTC for deterministic tests.
	loc := time.UTC

	tests := []struct {
		name string
		expr string
		from time.Time
		want time.Time
	}{
		{
			name: "every_5_min",
			expr: "*/5 * * * *",
			from: time.Date(2026, 1, 15, 10, 3, 0, 0, loc),
			want: time.Date(2026, 1, 15, 10, 5, 0, 0, loc),
		},
		{
			name: "hourly",
			expr: "0 * * * *",
			from: time.Date(2026, 1, 15, 10, 30, 0, 0, loc),
			want: time.Date(2026, 1, 15, 11, 0, 0, 0, loc),
		},
		{
			name: "daily_2am",
			expr: "0 2 * * *",
			from: time.Date(2026, 1, 15, 10, 0, 0, 0, loc),
			want: time.Date(2026, 1, 16, 2, 0, 0, 0, loc),
		},
		{
			name: "monthly_1st",
			expr: "0 0 1 * *",
			from: time.Date(2026, 1, 15, 0, 0, 0, 0, loc),
			want: time.Date(2026, 2, 1, 0, 0, 0, 0, loc),
		},
		{
			name: "weekday_9am",
			expr: "0 9 * * 1-5",
			from: time.Date(2026, 1, 16, 10, 0, 0, 0, loc), // Friday
			want: time.Date(2026, 1, 19, 9, 0, 0, 0, loc), // next Monday 9am
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextCron(tt.expr, tt.from)
			if !got.Equal(tt.want) {
				t.Errorf("nextCron(%q, %v) = %v, want %v", tt.expr, tt.from, got, tt.want)
			}
		})
	}
}

func TestStoreCreateAndGet(t *testing.T) {
	store := newTestStore(t)

	job, err := store.Create(&Job{
		Name:     "test-job",
		Schedule: "*/5 * * * *",
		URL:      "https://example.com/hook",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if job.ID == "" {
		t.Fatal("expected non-empty ID")
	}

	got, err := store.Get(job.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Name != "test-job" {
		t.Errorf("expected name %q, got %q", "test-job", got.Name)
	}
}

func TestStoreList(t *testing.T) {
	store := newTestStore(t)

	_, _ = store.Create(&Job{Name: "zebra", Schedule: "0 * * * *", URL: "http://z", Enabled: true})
	_, _ = store.Create(&Job{Name: "alpha", Schedule: "0 * * * *", URL: "http://a", Enabled: true})

	jobs, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if jobs[0].Name != "alpha" || jobs[1].Name != "zebra" {
		t.Errorf("jobs not sorted by name: %s, %s", jobs[0].Name, jobs[1].Name)
	}
}

func TestStoreDelete(t *testing.T) {
	store := newTestStore(t)

	job, _ := store.Create(&Job{Name: "temp", Schedule: "0 * * * *", URL: "http://t", Enabled: true})
	if err := store.Delete(job.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := store.Get(job.ID)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestStoreUpdate(t *testing.T) {
	store := newTestStore(t)

	job, _ := store.Create(&Job{Name: "original", Schedule: "0 * * * *", URL: "http://o", Enabled: true})
	updated, err := store.Update(&Job{
		ID:       job.ID,
		Name:     "updated",
		Schedule: "*/10 * * * *",
		URL:      "http://u",
		Enabled:  false,
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Name != "updated" {
		t.Errorf("expected name %q, got %q", "updated", updated.Name)
	}
	if updated.Enabled {
		t.Error("expected Enabled=false")
	}
}

func TestStoreValidation(t *testing.T) {
	store := newTestStore(t)

	_, err := store.Create(&Job{Name: "", Schedule: "0 * * * *", URL: "http://x", Enabled: true})
	if err == nil {
		t.Error("expected error for empty name")
	}

	_, err = store.Create(&Job{Name: "test", Schedule: "", URL: "http://x", Enabled: true})
	if err == nil {
		t.Error("expected error for empty schedule")
	}

	_, err = store.Create(&Job{Name: "test", Schedule: "invalid", URL: "http://x", Enabled: true})
	if err == nil {
		t.Error("expected error for invalid schedule")
	}
}
