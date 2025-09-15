package tables

import (
    "encoding/json"
    "net/http"

    "github.com/go-chi/chi/v5"

    "yourapp/internal/auth"
    httpserver "yourapp/internal/http"
    "yourapp/internal/repo"
    "yourapp/internal/models"
)

type Handler struct {
	repo repo.Repo
}

func New(repo repo.Repo) *Handler { return &Handler{repo: repo} }

// Search handles POST /tables/{table}/search with JSON payload containing page/filterFields
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	// Require org context if needed later; currently generic search is not org-scoped in DB
    orgID, ok := auth.OrgFromContext(r.Context())
    if !ok {
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

    // Fetch schema first, then rows
    schema, err := h.repo.GetUserTableSchema(r.Context(), orgID, table)
    if err != nil {
        httpserver.JSON(w, http.StatusInternalServerError, map[string]string{"error": "schema fetch failed"})
        return
    }
    rows, err := h.repo.SearchUserTable(r.Context(), orgID, table, payload)
    if err != nil {
        httpserver.JSON(w, http.StatusInternalServerError, map[string]string{"error": "search failed"})
        return
    }
    httpserver.JSON(w, http.StatusOK, map[string]any{"columns": schema, "content": rows})
}

// Create handles POST /tables with JSON body {"name": "..."} to create a new table for the org
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
    orgID, ok := auth.OrgFromContext(r.Context())
    if !ok {
        httpserver.JSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
        return
    }
    defer r.Body.Close()
    var body struct{ Name string `json:"name"` }
    dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
    if err := dec.Decode(&body); err != nil || body.Name == "" {
        httpserver.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON or missing name"})
        return
    }
    table, created, err := h.repo.CreateUserTable(r.Context(), orgID, body.Name)
    if err != nil {
        httpserver.JSON(w, http.StatusConflict, map[string]string{"error": "create failed"})
        return
    }
    httpserver.JSON(w, http.StatusCreated, map[string]any{"created": created, "table": table})
}

// List handles GET /tables to list org-scoped user-defined tables
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
    orgID, ok := auth.OrgFromContext(r.Context())
    if !ok {
        httpserver.JSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
        return
    }
    tables, err := h.repo.ListUserTables(r.Context(), orgID)
    if err != nil {
        httpserver.JSON(w, http.StatusInternalServerError, map[string]string{"error": "list failed"})
        return
    }
    httpserver.JSON(w, http.StatusOK, map[string]any{"tables": tables})
}

// AddColumn handles POST /tables/{table}/columns to add a column to a user-defined table
func (h *Handler) AddColumn(w http.ResponseWriter, r *http.Request) {
    orgID, ok := auth.OrgFromContext(r.Context())
    if !ok {
        httpserver.JSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
        return
    }
    table := chi.URLParam(r, "table")
    if table == "" {
        httpserver.JSON(w, http.StatusBadRequest, map[string]string{"error": "missing table"})
        return
    }
    defer r.Body.Close()
    var input models.TableColumnInput
    dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
    if err := dec.Decode(&input); err != nil {
        httpserver.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
        return
    }
    if input.Name == "" || input.Type == "" {
        httpserver.JSON(w, http.StatusBadRequest, map[string]string{"error": "name and type are required"})
        return
    }
    col, created, err := h.repo.AddUserTableColumn(r.Context(), orgID, table, input)
    if err != nil {
        httpserver.JSON(w, http.StatusConflict, map[string]string{"error": "add column failed"})
        return
    }
    status := http.StatusCreated
    if !created { status = http.StatusOK }
    httpserver.JSON(w, status, map[string]any{"created": created, "column": col})
}

// AddRow handles POST /tables/{table}/rows to insert a row with JSON body values
func (h *Handler) AddRow(w http.ResponseWriter, r *http.Request) {
    orgID, ok := auth.OrgFromContext(r.Context())
    if !ok {
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
        httpserver.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
        return
    }
    payload, err := json.Marshal(body)
    if err != nil {
        httpserver.JSON(w, http.StatusBadRequest, map[string]string{"error": "failed to encode values"})
        return
    }
    row, err := h.repo.InsertUserTableRow(r.Context(), orgID, table, payload)
    if err != nil {
        httpserver.JSON(w, http.StatusConflict, map[string]string{"error": "insert failed"})
        return
    }
    httpserver.JSON(w, http.StatusCreated, map[string]any{"row": row})
}
