-- name: GetMappedRolesForGroups :many
SELECT role_name
FROM idp_group_role_mappings
WHERE org_id = $1
  AND provider = $2
  AND idp_group_id = ANY(sqlc.arg(group_ids)::text[]);
