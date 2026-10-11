package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/ketsuna-org/sovrabase/internal/mcp"
)

// mcpEndpointPath is where the MCP server is mounted on the main router.
const mcpEndpointPath = "/mcp"

func (a *AdminServer) mcp() *mcp.Server {
	return mcpServerFor(a.projects, a.dataDir)
}

type mcpConfigResponse struct {
	Enabled       bool             `json:"enabled"`
	Endpoint      string           `json:"endpoint"`
	DisabledTools []string         `json:"disabled_tools"`
	Tools         []mcp.ToolStatus `json:"tools"`
}

type mcpTokenResponse struct {
	Token      string   `json:"token,omitempty"`
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Prefix     string   `json:"prefix"`
	Scopes     []string `json:"scopes"`
	CreatedAt  string   `json:"created_at"`
	LastUsedAt *string  `json:"last_used_at"`
}

type mcpCallResponse struct {
	At         string `json:"at"`
	TokenName  string `json:"token_name"`
	Tool       string `json:"tool"`
	OK         bool   `json:"ok"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error"`
}

func mcpConfigPayload(cfg mcp.Config) mcpConfigResponse {
	disabled := cfg.DisabledTools
	if disabled == nil {
		disabled = []string{}
	}
	return mcpConfigResponse{
		Enabled:       cfg.Enabled,
		Endpoint:      mcpEndpointPath,
		DisabledTools: disabled,
		Tools:         mcp.ToolStatuses(cfg),
	}
}

func mcpTokenPayload(t *mcp.Token) mcpTokenResponse {
	resp := mcpTokenResponse{
		ID:        t.ID,
		Name:      t.Name,
		Prefix:    t.Prefix,
		Scopes:    t.Scopes,
		CreatedAt: t.CreatedAt.UTC().Format(time.RFC3339),
	}
	if resp.Scopes == nil {
		resp.Scopes = []string{}
	}
	if t.LastUsedAt != nil {
		s := t.LastUsedAt.UTC().Format(time.RFC3339)
		resp.LastUsedAt = &s
	}
	return resp
}

// handleAdminGetMCP returns a project's MCP settings and tool catalogue.
func (a *AdminServer) handleAdminGetMCP(w http.ResponseWriter, r *http.Request) {
	store, err := a.mcp().Store(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	writeJSON(w, http.StatusOK, mcpConfigPayload(store.Config()))
}

// handleAdminSetMCP enables/disables MCP and sets the disabled tool list.
func (a *AdminServer) handleAdminSetMCP(w http.ResponseWriter, r *http.Request) {
	store, err := a.mcp().Store(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	var req struct {
		Enabled       *bool     `json:"enabled"`
		DisabledTools *[]string `json:"disabled_tools"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	cfg := store.Config()
	if req.Enabled != nil {
		cfg.Enabled = *req.Enabled
	}
	if req.DisabledTools != nil {
		names, err := mcp.NormalizeDisabledTools(*req.DisabledTools)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		cfg.DisabledTools = names
	}
	if err := store.SetConfig(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, mcpConfigPayload(store.Config()))
}

// handleAdminListMCPTokens lists a project's MCP tokens (never the secret).
func (a *AdminServer) handleAdminListMCPTokens(w http.ResponseWriter, r *http.Request) {
	store, err := a.mcp().Store(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	tokens := store.ListTokens()
	out := make([]mcpTokenResponse, 0, len(tokens))
	for i := range tokens {
		out = append(out, mcpTokenPayload(&tokens[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

// handleAdminCreateMCPToken mints a token; the plaintext is returned once.
func (a *AdminServer) handleAdminCreateMCPToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string   `json:"name"`
		Scopes []string `json:"scopes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	projectID := r.PathValue("id")
	if _, err := a.mcp().Store(projectID); err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	t, plain, err := a.mcp().CreateToken(projectID, req.Name, req.Scopes)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp := mcpTokenPayload(t)
	resp.Token = plain
	writeJSON(w, http.StatusCreated, resp)
}

// handleAdminDeleteMCPToken revokes a token.
func (a *AdminServer) handleAdminDeleteMCPToken(w http.ResponseWriter, r *http.Request) {
	err := a.mcp().DeleteToken(r.PathValue("id"), r.PathValue("tokenId"))
	if err != nil {
		if errors.Is(err, mcp.ErrTokenNotFound) {
			writeError(w, http.StatusNotFound, "token not found")
			return
		}
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleAdminListMCPCalls returns recent MCP tool calls, newest first.
func (a *AdminServer) handleAdminListMCPCalls(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if _, err := a.mcp().Store(projectID); err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 200 {
		limit = 200
	}
	calls := a.mcp().Calls(projectID, limit)
	out := make([]mcpCallResponse, 0, len(calls))
	for _, c := range calls {
		out = append(out, mcpCallResponse{
			At:         c.At.UTC().Format(time.RFC3339),
			TokenName:  c.TokenName,
			Tool:       c.Tool,
			OK:         c.OK,
			DurationMs: c.DurationMs,
			Error:      c.Error,
		})
	}
	writeJSON(w, http.StatusOK, out)
}
