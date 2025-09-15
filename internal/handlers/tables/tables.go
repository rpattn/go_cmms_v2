package tables

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"yourapp/internal/auth"
	httpserver "yourapp/internal/http"
	"yourapp/internal/repo"
)

type Handler struct {
	repo repo.Repo
}

func New(repo repo.Repo) *Handler { return &Handler{repo: repo} }

// Search handles POST /tables/{table}/search with JSON payload containing page/filterFields
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	// Require org context if needed later; currently generic search is not org-scoped in DB
	if _, ok := auth.OrgFromContext(r.Context()); !ok {
		httpserver.JSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	table := chi.URLParam(r, "table")
	if table == "" {
		httpserver.JSON(w, http.StatusBadRequest, map[string]string{"error": "missing table"})
		return
	}

	defer r.Body.Close()
	var body map[string]any
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
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

	rows, err := h.repo.SearchUserTable(r.Context(), table, payload)
	if err != nil {
		httpserver.JSON(w, http.StatusInternalServerError, map[string]string{"error": "search failed"})
		return
	}
	httpserver.JSON(w, http.StatusOK, map[string]any{"content": rows})
}
