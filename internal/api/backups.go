package api

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ketsuna-org/sovrabase/internal/config"
	"github.com/ketsuna-org/sovrabase/internal/tenant"
)

// BackupsHandler manages backup creation, listing, deletion, and download.
type BackupsHandler struct {
	projects *tenant.ProjectManager
	dataDir  string
	cfg      *config.Config
}

// NewBackupsHandler creates a new BackupsHandler.
func NewBackupsHandler(pm *tenant.ProjectManager, dataDir string, cfg *config.Config) *BackupsHandler {
	return &BackupsHandler{
		projects: pm,
		dataDir:  dataDir,
		cfg:      cfg,
	}
}

// backupDir returns the path to the backups directory.
func (h *BackupsHandler) backupDir() string {
	return filepath.Join(h.dataDir, "backups")
}

// backupInfo holds metadata about a single backup.
type backupInfo struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	CreatedAt string `json:"created_at"`
	SizeBytes int64  `json:"size_bytes"`
}

// HandleListBackups lists all backup directories, ordered newest first.
func (h *BackupsHandler) HandleListBackups(w http.ResponseWriter, r *http.Request) {
	backupRoot := h.backupDir()
	entries, err := os.ReadDir(backupRoot)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, []backupInfo{})
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list backups: %v", err))
		return
	}

	var backups []backupInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		backups = append(backups, backupInfo{
			Name:      entry.Name(),
			Path:      filepath.Join(backupRoot, entry.Name()),
			CreatedAt: info.ModTime().UTC().Format(time.RFC3339),
			SizeBytes: dirSize(filepath.Join(backupRoot, entry.Name())),
		})
	}

	// Sort by creation time descending (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt > backups[j].CreatedAt
	})

	writeJSON(w, http.StatusOK, backups)
}

// HandleCreateBackup triggers a manual backup via the ProjectManager.
func (h *BackupsHandler) HandleCreateBackup(w http.ResponseWriter, r *http.Request) {
	backupRoot := h.backupDir()
	if err := os.MkdirAll(backupRoot, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create backup dir: %v", err))
		return
	}

	timestamp := time.Now().UTC().Format("20060102T150405Z")
	backupName := "backup-" + timestamp
	destDir := filepath.Join(backupRoot, backupName)

	if err := h.projects.Backup(destDir); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("backup failed: %v", err))
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status": "created",
		"name":   backupName,
		"path":   destDir,
	})
}

// HandleDeleteBackup removes a backup directory by name.
func (h *BackupsHandler) HandleDeleteBackup(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "backup name is required")
		return
	}

	// Prevent directory traversal
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		writeError(w, http.StatusBadRequest, "invalid backup name")
		return
	}

	backupPath := filepath.Join(h.backupDir(), name)

	// Verify the path is within the backups directory
	absBackupRoot, _ := filepath.Abs(h.backupDir())
	absTarget, _ := filepath.Abs(backupPath)
	if !strings.HasPrefix(absTarget, absBackupRoot) {
		writeError(w, http.StatusBadRequest, "invalid backup path")
		return
	}

	if err := os.RemoveAll(backupPath); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to delete backup: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// HandleDownloadBackup creates a zip archive of the backup folder on-the-fly and streams it.
func (h *BackupsHandler) HandleDownloadBackup(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "backup name is required")
		return
	}

	// Prevent directory traversal
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		writeError(w, http.StatusBadRequest, "invalid backup name")
		return
	}

	backupPath := filepath.Join(h.backupDir(), name)

	// Verify the path is within the backups directory
	absBackupRoot, _ := filepath.Abs(h.backupDir())
	absTarget, _ := filepath.Abs(backupPath)
	if !strings.HasPrefix(absTarget, absBackupRoot) {
		writeError(w, http.StatusBadRequest, "invalid backup path")
		return
	}

	// Verify the backup directory exists
	info, err := os.Stat(backupPath)
	if err != nil || !info.IsDir() {
		writeError(w, http.StatusNotFound, fmt.Sprintf("backup %q not found", name))
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, name))

	zw := zip.NewWriter(w)
	defer zw.Close()

	err = filepath.Walk(backupPath, func(filePath string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(backupPath, filePath)
		if err != nil {
			return err
		}

		if fi.IsDir() {
			// Add a directory entry for empty dirs
			_, err := zw.Create(relPath + "/")
			return err
		}

		header, err := zip.FileInfoHeader(fi)
		if err != nil {
			return err
		}
		header.Name = relPath
		header.Method = zip.Deflate

		writer, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}

		f, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(writer, f)
		return err
	})
	if err != nil {
		// We can't send an error header at this point because the response is already being streamed.
		// Abort by closing the zip writer with error; client will see a truncated archive.
		return
	}
}
