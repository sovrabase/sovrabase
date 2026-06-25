package api

import (
	"encoding/json"
	"net/http"

	"github.com/ketsuna-org/sovrabase/internal/emailtemplates"
)

func (a *AdminServer) getProjectEmailTemplateStore(r *http.Request) (*emailtemplates.Store, error) {
	env, err := a.getProjectEnv(r)
	if err != nil {
		return nil, err
	}
	return emailtemplates.NewStore(env.Engine.DB()), nil
}

func (a *AdminServer) handleAdminListEmailTemplates(w http.ResponseWriter, r *http.Request) {
	store, err := a.getProjectEmailTemplateStore(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	templates, err := store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if templates == nil {
		templates = []*emailtemplates.Template{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  templates,
		"count": len(templates),
	})
}

func (a *AdminServer) handleAdminSetEmailTemplate(w http.ResponseWriter, r *http.Request) {
	store, err := a.getProjectEmailTemplateStore(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	var tmpl emailtemplates.Template
	if err := json.NewDecoder(r.Body).Decode(&tmpl); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if tmpl.Body == "" {
		writeError(w, http.StatusBadRequest, "body is required")
		return
	}
	saved, err := store.Set(&tmpl)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (a *AdminServer) handleAdminResetEmailTemplate(w http.ResponseWriter, r *http.Request) {
	store, err := a.getProjectEmailTemplateStore(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	tmplType := emailtemplates.TemplateType(r.PathValue("type"))
	if err := store.Reset(tmplType); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}
