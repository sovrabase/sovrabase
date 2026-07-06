//go:build !windows

package main

import "log/slog"

func killStaleInstances(_ *slog.Logger) {}
