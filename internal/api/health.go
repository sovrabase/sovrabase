package api

import (
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"golang.org/x/sys/windows"

	"github.com/ketsuna-org/sovrabase/internal/config"
	"github.com/ketsuna-org/sovrabase/internal/db"
	"github.com/ketsuna-org/sovrabase/internal/tenant"
)

var startTime time.Time

func init() {
	startTime = time.Now()
}

// DeepHealthCheck returns an http.HandlerFunc that performs a comprehensive
// health check of the server and returns JSON with detailed status information.
func DeepHealthCheck(cfg *config.Config, pm *tenant.ProjectManager, dbEngine *db.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Database check
		dbStatus := "ok"
		dbMessage := "connected"
		if dbEngine != nil {
			// Try a simple operation to verify DB connectivity.
			cols, err := dbEngine.ListCollections()
			if err != nil {
				dbStatus = "error"
				dbMessage = err.Error()
			}
			_ = cols
		} else {
			dbStatus = "error"
			dbMessage = "db engine is nil"
		}

		// Disk check (Windows-compatible)
		diskInfo := getDiskUsage(cfg.DataDir)

		// Memory stats
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)

		// Projects
		projectCount := 0
		if pm != nil {
			projectCount = pm.ProjectCount()
		}

		overallStatus := "ok"
		if dbStatus != "ok" || (diskInfo != nil && diskInfo.UsedPercent > 95) {
			overallStatus = "degraded"
		}

		result := map[string]interface{}{
			"status":    overallStatus,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"uptime":    time.Since(startTime).String(),
			"version":   buildVersion(),
			"go_version": runtime.Version(),
			"database": map[string]interface{}{
				"status":  dbStatus,
				"message": dbMessage,
			},
			"disk": diskInfo,
			"memory": map[string]interface{}{
				"alloc_mb":       mem.Alloc / 1024 / 1024,
				"total_alloc_mb": mem.TotalAlloc / 1024 / 1024,
				"sys_mb":         mem.Sys / 1024 / 1024,
				"num_gc":         mem.NumGC,
			},
			"projects": map[string]interface{}{
				"active": projectCount,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(result)
	}
}

// diskUsage holds information about a disk partition.
type diskUsage struct {
	Path        string  `json:"path"`
	TotalBytes  uint64  `json:"total_bytes"`
	FreeBytes   uint64  `json:"free_bytes"`
	UsedPercent float64 `json:"used_percent"`
}

// getDiskUsage returns disk usage information for the given path.
// Uses windows.GetDiskFreeSpaceEx on Windows.
func getDiskUsage(path string) *diskUsage {
	// Ensure the path exists
	info, err := os.Stat(path)
	if err != nil {
		return &diskUsage{
			Path:        path,
			TotalBytes:  0,
			FreeBytes:   0,
			UsedPercent: 0,
		}
	}

	dir := path
	if !info.IsDir() {
		dir = info.Name()
	}

	// Use the parent directory if available
	absPath, err := os.Getwd()
	if err != nil {
		absPath = dir
	}

	// Use Windows API for disk space
	abs, err := windows.UTF16PtrFromString(absPath)
	if err != nil {
		return &diskUsage{Path: path}
	}

	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	err = windows.GetDiskFreeSpaceEx(abs, &freeBytesAvailable, &totalBytes, &totalFreeBytes)
	if err != nil {
		return &diskUsage{Path: path}
	}

	used := totalBytes - totalFreeBytes
	var usedPercent float64
	if totalBytes > 0 {
		usedPercent = float64(used) / float64(totalBytes) * 100.0
	}

	return &diskUsage{
		Path:        path,
		TotalBytes:  totalBytes,
		FreeBytes:   totalFreeBytes,
		UsedPercent: usedPercent,
	}
}

// buildVersion returns the build version from module info if available.
func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "dev"
}
