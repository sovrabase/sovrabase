package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// metadataSuffix is appended to the stored-file name to form the
// metadata file name (e.g. "photo.jpg.meta.json").
const metadataSuffix = ".meta.json"

// loadMetadata reads the JSON-encoded FileInfo stored alongside the
// file at diskPath (diskPath is the *data* file, not the meta file).
func loadMetadata(diskPath string) (*FileInfo, error) {
	metaPath := metaPathFor(diskPath)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}
	var info FileInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// saveMetadata persists info as JSON alongside the file at diskPath.
func saveMetadata(diskPath string, info *FileInfo) error {
	// Ensure timestamps are set if zero-valued.
	now := time.Now().UTC()
	if info.CreatedAt.IsZero() {
		info.CreatedAt = now
	}
	if info.UpdatedAt.IsZero() {
		info.UpdatedAt = now
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(metaPathFor(diskPath), data, 0o644)
}

// removeMetadata deletes the metadata file for the given data file.
func removeMetadata(diskPath string) error {
	return os.Remove(metaPathFor(diskPath))
}

// metaPathFor returns the metadata file path for a given data file path.
func metaPathFor(diskPath string) string {
	return diskPath + metadataSuffix
}

// listMetadata walks the given directory and collects FileInfo for
// every metadata file found.
func listMetadata(dir string) ([]FileInfo, error) {
	var results []FileInfo

	err := filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".json" {
			return nil
		}
		// Only pick up files ending in .meta.json
		if filepath.Ext(path[:len(path)-5]) != ".meta" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var info FileInfo
		if err := json.Unmarshal(data, &info); err != nil {
			// Skip files that aren't valid metadata (e.g. .json files
			// that happen to be stored in buckets).
			return nil
		}
		results = append(results, info)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}
