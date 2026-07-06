package api

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/ketsuna-org/sovrabase/internal/functions"
	"github.com/ketsuna-org/sovrabase/internal/metering"
	"github.com/ketsuna-org/sovrabase/internal/storage"
	"github.com/ketsuna-org/sovrabase/internal/tenant"
)

// projectHostAPI implements functions.HostAPI using the project engine.
type projectHostAPI struct {
	projectID string
	projects  *tenant.ProjectManager
	meters    *metering.MeterStore
}

func newProjectHostAPI(projectID string, pm *tenant.ProjectManager, meters *metering.MeterStore) functions.HostAPI {
	return &projectHostAPI{projectID: projectID, projects: pm, meters: meters}
}

func (h *projectHostAPI) engine() (*tenant.ProjectEnv, error) {
	return h.projects.GetProjectEnv(h.projectID)
}

func (h *projectHostAPI) DBGet(collection, id string) (map[string]interface{}, error) {
	env, err := h.engine()
	if err != nil {
		return nil, err
	}
	doc, err := env.Engine.Get(collection, id)
	if err != nil {
		return nil, err
	}
	return docToMap(doc), nil
}

func (h *projectHostAPI) DBCreate(collection string, data map[string]interface{}) (map[string]interface{}, error) {
	env, err := h.engine()
	if err != nil {
		return nil, err
	}
	id := uuid.New().String()
	if err := env.Engine.Insert(collection, id, data); err != nil {
		return nil, err
	}
	doc, err := env.Engine.Get(collection, id)
	if err != nil {
		return map[string]interface{}{"_id": id}, nil
	}
	return docToMap(doc), nil
}

func (h *projectHostAPI) DBUpdate(collection, id string, data map[string]interface{}) (map[string]interface{}, error) {
	env, err := h.engine()
	if err != nil {
		return nil, err
	}
	if err := env.Engine.Update(collection, id, data); err != nil {
		return nil, err
	}
	doc, err := env.Engine.Get(collection, id)
	if err != nil {
		return nil, err
	}
	return docToMap(doc), nil
}

func (h *projectHostAPI) DBDelete(collection, id string) error {
	env, err := h.engine()
	if err != nil {
		return err
	}
	return env.Engine.Delete(collection, id)
}

func (h *projectHostAPI) Fetch(ctx context.Context, method, url string, headers map[string]string, body interface{}) (int, []byte, error) {
	return functions.FetchHTTP(ctx, method, url, headers, body)
}

func (h *projectHostAPI) StorageUpload(ctx context.Context, bucket, path, contentType string, data []byte) (map[string]interface{}, error) {
	if len(data) > functions.StorageRPCMaxBytes {
		return nil, fmt.Errorf("storage: payload exceeds %d bytes", functions.StorageRPCMaxBytes)
	}
	env, err := h.engine()
	if err != nil {
		return nil, err
	}
	if err := h.checkStorageQuota(len(data)); err != nil {
		return nil, err
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	info, err := env.Storage.Upload(ctx, bucket, path, bytes.NewReader(data), contentType)
	if err != nil {
		return nil, err
	}
	if h.meters != nil && info != nil {
		_ = h.meters.Inc(h.projectID, metering.MetricBandwidthUp, info.Size)
		_ = h.meters.Inc(h.projectID, metering.MetricStorageBytes, info.Size)
	}
	return fileInfoToMap(info), nil
}

func (h *projectHostAPI) StorageDownload(ctx context.Context, bucket, path string) ([]byte, map[string]interface{}, error) {
	env, err := h.engine()
	if err != nil {
		return nil, nil, err
	}
	rc, info, err := env.Storage.Download(ctx, bucket, path)
	if err != nil {
		return nil, nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, functions.StorageRPCMaxBytes+1))
	if err != nil {
		return nil, nil, err
	}
	if len(data) > functions.StorageRPCMaxBytes {
		return nil, nil, fmt.Errorf("storage: download exceeds %d bytes", functions.StorageRPCMaxBytes)
	}
	if h.meters != nil && info != nil {
		_ = h.meters.Inc(h.projectID, metering.MetricBandwidthDown, int64(len(data)))
	}
	return data, fileInfoToMap(info), nil
}

func (h *projectHostAPI) StorageDelete(ctx context.Context, bucket, path string) error {
	env, err := h.engine()
	if err != nil {
		return err
	}
	rc, info, err := env.Storage.Download(ctx, bucket, path)
	if err != nil {
		return err
	}
	_ = rc.Close()
	if err := env.Storage.Delete(ctx, bucket, path); err != nil {
		return err
	}
	if h.meters != nil && info != nil {
		_ = h.meters.Inc(h.projectID, metering.MetricStorageBytes, -info.Size)
	}
	return nil
}

func (h *projectHostAPI) StorageList(ctx context.Context, bucket, prefix string) ([]map[string]interface{}, error) {
	env, err := h.engine()
	if err != nil {
		return nil, err
	}
	files, err := env.Storage.List(ctx, bucket, prefix)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(files))
	for i := range files {
		out = append(out, fileInfoToMap(&files[i]))
	}
	return out, nil
}

func (h *projectHostAPI) checkStorageQuota(uploadSize int) error {
	proj, err := h.projects.GetProject(h.projectID)
	if err != nil || proj == nil || proj.StorageQuota <= 0 {
		return nil
	}
	var currentUsage int64
	if h.meters != nil {
		currentUsage, _ = h.meters.GetStorageUsage(h.projectID)
	}
	if currentUsage+int64(uploadSize) > proj.StorageQuota {
		return fmt.Errorf("storage quota exceeded (used %d/%d bytes, attempting to upload %d bytes)", currentUsage, proj.StorageQuota, uploadSize)
	}
	return nil
}

func fileInfoToMap(info *storage.FileInfo) map[string]interface{} {
	if info == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"bucket":       info.Bucket,
		"path":         info.Path,
		"size":         info.Size,
		"content_type": info.ContentType,
		"created_at":   info.CreatedAt,
		"updated_at":   info.UpdatedAt,
		"url":          info.URL,
	}
}

func docToMap(doc map[string]interface{}) map[string]interface{} {
	if doc == nil {
		return map[string]interface{}{}
	}
	return doc
}

// executorHostResolver returns host API for a project during execution.
func executorHostResolver(pm *tenant.ProjectManager, projectID string, meters *metering.MeterStore) functions.HostAPI {
	return newProjectHostAPI(projectID, pm, meters)
}
