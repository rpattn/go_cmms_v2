// internal/handlers/workorders/controller.go
package users

import (
	"encoding/json"
	"net/http"

	"yourapp/internal/auth"
	httpserver "yourapp/internal/http"
	"yourapp/internal/repo"
)

type Handler struct {
	repo repo.Repo
}

func New(repo repo.Repo) *Handler {
	return &Handler{repo: repo}
}

// Search handles searching for users based on query parameters.
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	// Get org_id from context (set by middleware)
	org_id, ok := auth.OrgFromContext(r.Context())
	if !ok {
		httpserver.JSON(w, http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
		})
		return
	}

	// Get search query from URL parameter
	// Decode patch JSON (empty object {} is OK)
	defer r.Body.Close()
	var body map[string]any
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)) // 1MB
	// dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		httpserver.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if dec.More() {
		httpserver.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON (extra content)"})
		return
	}

	payload, err := json.Marshal(body)
	if err != nil {
		httpserver.JSON(w, http.StatusBadRequest, map[string]string{"error": "failed to encode payload"})
		return
	}

	users, err := h.repo.SearchUsers(r.Context(), org_id, payload)
	if err != nil {
		httpserver.JSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to search users",
		})
		return
	}
	httpserver.JSON(w, http.StatusOK, map[string]any{
		"content": users,
	})
}
