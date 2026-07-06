package scheduler

import (
	"time"

	"github.com/ketsuna-org/sovrabase/internal/cronexpr"
)

func validateCron(expr string) error {
	return cronexpr.Validate(expr)
}

func nextCron(expr string, from time.Time) time.Time {
	return cronexpr.Next(expr, from)
}
