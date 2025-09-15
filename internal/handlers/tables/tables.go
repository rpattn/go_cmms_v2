package tables

import (
    "encoding/json"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"

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
    // Unpack data maps and promote total_count to top-level
    contents := make([]map[string]any, 0, len(rows))
    var totalCount int64
    for i, r := range rows {
        if i == 0 { totalCount = r.TotalCount }
        if r.Data == nil {
            contents = append(contents, map[string]any{})
            continue
        }
        // Copy map to avoid unexpected aliasing
        m := make(map[string]any, len(r.Data))
        for k, v := range r.Data { m[k] = v }
        // Optionally drop duplicate id fields if present (we're not returning row_id separately)
        // If you want to keep a specific key, adjust here.
        contents = append(contents, m)
    }
    httpserver.JSON(w, http.StatusOK, map[string]any{"columns": schema, "content": contents, "total_count": totalCount})
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
        status, msg := httpserver.PGErrorMessage(err, "create failed")
        httpserver.JSON(w, status, map[string]string{"error": msg})
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

// Delete handles DELETE /tables/{table} for the current org
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
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
    ut, deleted, err := h.repo.DeleteUserTable(r.Context(), orgID, table)
    if err != nil {
        status, msg := httpserver.PGErrorMessage(err, "delete failed")
        httpserver.JSON(w, status, map[string]string{"error": msg})
        return
    }
    if !deleted {
        httpserver.JSON(w, http.StatusNotFound, map[string]string{"error": "table not found"})
        return
    }
    httpserver.JSON(w, http.StatusOK, map[string]any{"deleted": true, "table": ut})
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
        status, msg := httpserver.PGErrorMessage(err, "add column failed")
        httpserver.JSON(w, status, map[string]string{"error": msg})
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
        status, msg := httpserver.PGErrorMessage(err, "insert failed")
        httpserver.JSON(w, status, map[string]string{"error": msg})
        return
    }
    httpserver.JSON(w, http.StatusCreated, map[string]any{"row": row})
}

// LookupRow handles POST /tables/rows/lookup with JSON body {"id":"<uuid>"}
// Returns the EAV-composed JSON for that row id using app.row_to_json.
func (h *Handler) LookupRow(w http.ResponseWriter, r *http.Request) {
    orgID, ok := auth.OrgFromContext(r.Context())
    if !ok {
        httpserver.JSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
        return
    }
    defer r.Body.Close()
    var body struct{ ID string `json:"id"` }
    dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
    if err := dec.Decode(&body); err != nil || body.ID == "" {
        httpserver.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON or missing id"})
        return
    }
    uid, err := uuid.Parse(body.ID)
    if err != nil {
        httpserver.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid UUID"})
        return
    }
    data, found, err := h.repo.GetRowData(r.Context(), orgID, uid)
    if err != nil {
        status, msg := httpserver.PGErrorMessage(err, "lookup failed")
        httpserver.JSON(w, status, map[string]string{"error": msg})
        return
    }
    if !found {
        httpserver.JSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
        return
    }
    httpserver.JSON(w, http.StatusOK, map[string]any{"data": data})
}

// LookupIndexed handles POST /tables/{table}/rows/indexed
// Body: {"field":"title","q":"fil","limit":20}
// Returns items: [{id, label}...]
func (h *Handler) LookupIndexed(w http.ResponseWriter, r *http.Request) {
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
    var body struct{
        Field *string `json:"field"`
        Q     *string `json:"q"`
        Limit *int    `json:"limit"`
    }
    dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
    if err := dec.Decode(&body); err != nil {
        httpserver.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
        return
    }
    lim := 20
    if body.Limit != nil { lim = *body.Limit }
    items, err := h.repo.LookupIndexedRows(r.Context(), orgID, table, body.Field, body.Q, lim)
    if err != nil {
        status, msg := httpserver.PGErrorMessage(err, "lookup failed")
        httpserver.JSON(w, status, map[string]string{"error": msg})
        return
    }
    httpserver.JSON(w, http.StatusOK, map[string]any{"items": items})
}

// IndexedFields handles GET /tables/indexed-fields to list org's indexed text/enum fields per table
func (h *Handler) IndexedFields(w http.ResponseWriter, r *http.Request) {
    orgID, ok := auth.OrgFromContext(r.Context())
    if !ok {
        httpserver.JSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
        return
    }
    items, err := h.repo.ListIndexedFields(r.Context(), orgID)
    if err != nil {
        status, msg := httpserver.PGErrorMessage(err, "fetch failed")
        httpserver.JSON(w, status, map[string]string{"error": msg})
        return
    }
    httpserver.JSON(w, http.StatusOK, map[string]any{"items": items})
}

// DeleteRow handles DELETE /tables/{table}/rows/{row_id}
func (h *Handler) DeleteRow(w http.ResponseWriter, r *http.Request) {
    orgID, ok := auth.OrgFromContext(r.Context())
    if !ok {
        httpserver.JSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
        return
    }
    table := chi.URLParam(r, "table")
    rowParam := chi.URLParam(r, "row_id")
    if table == "" || rowParam == "" {
        httpserver.JSON(w, http.StatusBadRequest, map[string]string{"error": "missing table or row_id"})
        return
    }
    rid, err := uuid.Parse(rowParam)
    if err != nil {
        httpserver.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid row_id"})
        return
    }
    deleted, err := h.repo.DeleteUserTableRow(r.Context(), orgID, table, rid)
    if err != nil {
        status, msg := httpserver.PGErrorMessage(err, "delete failed")
        httpserver.JSON(w, status, map[string]string{"error": msg})
        return
    }
    if !deleted {
        httpserver.JSON(w, http.StatusNotFound, map[string]string{"error": "row not found"})
        return
    }
    httpserver.JSON(w, http.StatusOK, map[string]any{"deleted": true, "row_id": rid.String()})
}

// RemoveColumn handles DELETE /tables/{table}/columns/{column}
func (h *Handler) RemoveColumn(w http.ResponseWriter, r *http.Request) {
    orgID, ok := auth.OrgFromContext(r.Context())
    if !ok {
        httpserver.JSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
        return
    }
    table := chi.URLParam(r, "table")
    column := chi.URLParam(r, "column")
    if table == "" || column == "" {
        httpserver.JSON(w, http.StatusBadRequest, map[string]string{"error": "missing table or column"})
        return
    }
    col, deleted, err := h.repo.RemoveUserTableColumn(r.Context(), orgID, table, column)
    if err != nil {
        status, msg := httpserver.PGErrorMessage(err, "delete failed")
        httpserver.JSON(w, status, map[string]string{"error": msg})
        return
    }
    if !deleted {
        httpserver.JSON(w, http.StatusNotFound, map[string]string{"error": "column not found"})
        return
    }
    httpserver.JSON(w, http.StatusOK, map[string]any{"deleted": true, "column": col})
}
