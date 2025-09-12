-- name: FindOrgBySlug :one
SELECT * FROM organisations WHERE slug = $1;

-- name: FindOrgByTenantID :one
SELECT * FROM organisations WHERE ms_tenant_id = $1;

-- name: FindOrgByID :one
SELECT * FROM organisations WHERE id = $1;

-- name: CreateOrg :one
INSERT INTO organisations (slug, name, ms_tenant_id)
VALUES ($1, $2, $3)
RETURNING *;
