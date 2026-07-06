//go:build windows

package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

// killStaleInstances force-kills any old sovrabase.exe process that may be
// holding PebbleDB file locks after an unclean Ctrl+C on Windows.
func killStaleInstances(logger *slog.Logger) {
	ourPID := os.Getpid()
	cmd := exec.Command("taskkill", "/F", "/IM", "sovrabase.exe", "/FI", fmt.Sprintf("PID ne %d", ourPID))
	out, _ := cmd.CombinedOutput()
	if strings.Contains(string(out), "SUCCESS") {
		logger.Info("Cleaned up stale sovrabase instance(s)")
	}
}
