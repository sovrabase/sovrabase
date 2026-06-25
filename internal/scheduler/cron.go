package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// cronField represents the bounds of a cron field.
type cronField struct {
	min, max int
}

var (
	fieldMinute  = cronField{0, 59}
	fieldHour    = cronField{0, 23}
	fieldDOM     = cronField{1, 31}
	fieldMonth   = cronField{1, 12}
	fieldDOW     = cronField{0, 6} // 0=Sunday
)

// cronSchedule holds the parsed set of allowed values for each field.
type cronSchedule struct {
	minutes    map[int]bool
	hours      map[int]bool
	dom        map[int]bool
	month      map[int]bool
	dow        map[int]bool
	domRestricted bool // true if DOM is not *
	dowRestricted bool // true if DOW is not *
}

// validateCron validates a 5-field cron expression.
func validateCron(expr string) error {
	_, err := parseCron(expr)
	return err
}

// parseCron parses a 5-field cron expression into a cronSchedule.
// Supports: *, */n, n, n-m (range), n,m (list).
func parseCron(expr string) (*cronSchedule, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("expected 5 fields, got %d", len(fields))
	}

	sched := &cronSchedule{
		minutes: make(map[int]bool),
		hours:   make(map[int]bool),
		dom:     make(map[int]bool),
		month:   make(map[int]bool),
		dow:     make(map[int]bool),
	}

	var err error
	sched.minutes, err = parseField(fields[0], fieldMinute)
	if err != nil {
		return nil, fmt.Errorf("minute field: %w", err)
	}
	sched.hours, err = parseField(fields[1], fieldHour)
	if err != nil {
		return nil, fmt.Errorf("hour field: %w", err)
	}
	sched.dom, err = parseField(fields[2], fieldDOM)
	if err != nil {
		return nil, fmt.Errorf("day-of-month field: %w", err)
	}
	sched.month, err = parseField(fields[3], fieldMonth)
	if err != nil {
		return nil, fmt.Errorf("month field: %w", err)
	}
	sched.dow, err = parseField(fields[4], fieldDOW)
	if err != nil {
		return nil, fmt.Errorf("day-of-week field: %w", err)
	}

	// Track whether DOM and DOW are restricted (not just *).
	sched.domRestricted = fields[2] != "*"
	sched.dowRestricted = fields[4] != "*"

	return sched, nil
}

// parseField parses a single cron field into a set of allowed values.
func parseField(s string, bounds cronField) (map[int]bool, error) {
	result := make(map[int]bool)

	for _, part := range strings.Split(s, ",") {
		if part == "*" {
			for i := bounds.min; i <= bounds.max; i++ {
				result[i] = true
			}
			continue
		}

		// Handle step: */n or n-m/s
		step := 1
		rangePart := part
		if idx := strings.Index(part, "/"); idx != -1 {
			s, err := strconv.Atoi(part[idx+1:])
			if err != nil || s < 1 {
				return nil, fmt.Errorf("invalid step in %q", part)
			}
			step = s
			rangePart = part[:idx]
		}

		lo, hi := bounds.min, bounds.max
		if rangePart != "*" {
			if idx := strings.Index(rangePart, "-"); idx != -1 {
				// Range: n-m
				parsedLo, err := strconv.Atoi(rangePart[:idx])
				if err != nil || parsedLo < bounds.min || parsedLo > bounds.max {
					return nil, fmt.Errorf("invalid range start in %q", part)
				}
				lo = parsedLo
				parsedHi, err := strconv.Atoi(rangePart[idx+1:])
				if err != nil || parsedHi < bounds.min || parsedHi > bounds.max {
					return nil, fmt.Errorf("invalid range end in %q", part)
				}
				hi = parsedHi
			} else {
				// Single value
				v, err := strconv.Atoi(rangePart)
				if err != nil || v < bounds.min || v > bounds.max {
					return nil, fmt.Errorf("invalid value %q", part)
				}
				lo = v
				hi = v
			}
		}

		for i := lo; i <= hi; i += step {
			result[i] = true
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("empty field")
	}
	return result, nil
}

// nextCron computes the next time the cron expression should fire after `from`.
func nextCron(expr string, from time.Time) time.Time {
	sched, err := parseCron(expr)
	if err != nil {
		return from.Add(time.Hour)
	}

	// Start from the next minute, truncated to minute boundary.
	t := from.Truncate(time.Minute).Add(time.Minute)

	limit := from.AddDate(2, 0, 0)

	for t.Before(limit) {
		if !sched.month[int(t.Month())] {
			t = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location()).AddDate(0, 1, 0)
			continue
		}

		// DOM/DOW matching: cron uses OR when both are restricted, AND otherwise.
		dayMatch := false
		if sched.domRestricted && sched.dowRestricted {
			dayMatch = sched.dom[t.Day()] || sched.dow[int(t.Weekday())]
		} else {
			dayMatch = sched.dom[t.Day()] && sched.dow[int(t.Weekday())]
		}
		if !dayMatch {
			t = t.AddDate(0, 0, 1)
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
			continue
		}
		if !sched.hours[t.Hour()] {
			t = t.Add(time.Hour)
			t = t.Truncate(time.Hour)
			continue
		}
		if !sched.minutes[t.Minute()] {
			t = t.Add(time.Minute)
			continue
		}
		return t
	}

	return from.Add(time.Hour)
}
