package repo

import (
    "context"
    "fmt"
    "log/slog"
    "time"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgtype"

    db "yourapp/internal/db/gen"
    "yourapp/internal/models"
)

// ---------------- Orgs & Memberships ----------------

func (p *pgRepo) FindOrgBySlug(ctx context.Context, slug string) (models.Org, error) {
    slog.DebugContext(ctx, "FindOrgBySlug", "slug", slug)
    o, err := p.q.FindOrgBySlug(ctx, slug)
    if err != nil {
        slog.ErrorContext(ctx, "FindOrgBySlug failed", "err", err)
        return models.Org{}, err
    }
    return models.Org{
        ID:       toUUID(o.ID),
        Slug:     o.Slug,
        Name:     o.Name,
        TenantID: fromText(o.MsTenantID),
    }, nil
}

func (p *pgRepo) FindOrgByID(ctx context.Context, id uuid.UUID) (models.Org, error) {
    slog.DebugContext(ctx, "FindOrgByID", "org_id", id.String())
    o, err := p.q.FindOrgByID(ctx, toPgUUID(id))
    if err != nil {
        slog.ErrorContext(ctx, "FindOrgByID failed", "err", err)
        return models.Org{}, err
    }
    return models.Org{
        ID:       toUUID(o.ID),
        Slug:     o.Slug,
        Name:     o.Name,
        TenantID: fromText(o.MsTenantID),
    }, nil
}

func (p *pgRepo) FindOrgByTenantID(ctx context.Context, tid string) (models.Org, error) {
    slog.DebugContext(ctx, "FindOrgByTenantID", "tenant_id", tid)
    o, err := p.q.FindOrgByTenantID(ctx, toText(tid))
    if err != nil {
        slog.ErrorContext(ctx, "FindOrgByTenantID failed", "err", err)
        return models.Org{}, err
    }
    return models.Org{
        ID:       toUUID(o.ID),
        Slug:     o.Slug,
        Name:     o.Name,
        TenantID: fromText(o.MsTenantID),
    }, nil
}

func (p *pgRepo) CreateOrg(ctx context.Context, slug, name, tenantID string) (models.Org, error) {
    slog.DebugContext(ctx, "CreateOrg", "slug", slug)
    o, err := p.q.CreateOrg(ctx, db.CreateOrgParams{
        Slug:       slug,
        Name:       name,
        MsTenantID: toNullableText(tenantID),
    })
    if err != nil {
        slog.ErrorContext(ctx, "CreateOrg failed", "err", err)
        return models.Org{}, err
    }
    return models.Org{
        ID:       toUUID(o.ID),
        Slug:     o.Slug,
        Name:     o.Name,
        TenantID: fromText(o.MsTenantID),
    }, nil
}

func (p *pgRepo) EnsureMembership(ctx context.Context, orgID, userID uuid.UUID, defaultRole models.OrgRole) (models.OrgRole, error) {
    slog.DebugContext(ctx, "EnsureMembership", "org_id", orgID.String(), "user_id", userID.String(), "default_role", string(defaultRole))
    roleText, err := p.q.EnsureMembership(ctx, db.EnsureMembershipParams{
        OrgID:  fromUUID(orgID),
        UserID: fromUUID(userID),
        Role:   string(defaultRole),
    })
    if err != nil {
        slog.ErrorContext(ctx, "EnsureMembership failed", "err", err)
        return "", fmt.Errorf("membership failed: %w", err)
    }
    return models.OrgRole(roleText), nil
}

func (p *pgRepo) GetRole(ctx context.Context, orgID, userID uuid.UUID) (models.OrgRole, error) {
    slog.DebugContext(ctx, "GetRole", "org_id", orgID.String(), "user_id", userID.String())
    roleStr, err := p.q.GetRole(ctx, db.GetRoleParams{
        OrgID:  fromUUID(orgID),
        UserID: fromUUID(userID),
    })
    if err != nil {
        slog.ErrorContext(ctx, "GetRole failed", "err", err)
        return "", err
    }
    return models.OrgRole(roleStr), nil
}

// Find best mapped role for given IdP groups (no persistence here).
func (p *pgRepo) ApplyGroupRoleMappings(ctx context.Context, orgID uuid.UUID, provider string, groupIDs []string) (models.OrgRole, error) {
    slog.DebugContext(ctx, "ApplyGroupRoleMappings", "org_id", orgID.String(), "provider", provider, "groups_count", len(groupIDs))
    if len(groupIDs) == 0 {
        return "", nil
    }
    rows, err := p.q.GetMappedRolesForGroups(ctx, db.GetMappedRolesForGroupsParams{
        OrgID:    fromUUID(orgID),
        Provider: provider,
        GroupIds: groupIDs,
    })
    if err != nil {
        slog.ErrorContext(ctx, "GetMappedRolesForGroups failed", "err", err)
        return "", err
    }

    best := ""
    for _, v := range rows {
        var role string
        switch x := v.(type) {
        case string:
            role = x
        case []byte:
            role = string(x)
        default:
            continue
        }
        if best == "" || rankRole(role) > rankRole(best) {
            best = role
        }
    }

    if best == "" {
        return "", nil
    }
    return models.OrgRole(best), nil
}

func (p *pgRepo) PickUserOrg(ctx context.Context, uid uuid.UUID) (models.Org, error) {
    slog.DebugContext(ctx, "PickUserOrg", "user_id", uid.String())
    o, err := p.q.PickUserOrg(ctx, fromUUID(uid))
    if err != nil {
        slog.ErrorContext(ctx, "PickUserOrg failed", "err", err)
        return models.Org{}, err
    }
    return models.Org{
        ID:       toUUID(o.ID),
        Slug:     o.Slug,
        Name:     o.Name,
        TenantID: fromText(o.MsTenantID),
    }, nil
}

func (p *pgRepo) ListUserOrgs(ctx context.Context, uid uuid.UUID) ([]models.OrgSummary, error) {
    rows, err := p.q.ListUserOrgs(ctx, fromUUID(uid))
    if err != nil {
        return nil, err
    }
    res := make([]models.OrgSummary, 0, len(rows))
    for _, r := range rows {
        res = append(res, models.OrgSummary{
            ID:        toUUID(r.ID),
            Slug:      r.Slug,
            Name:      r.Name,
            Role:      models.OrgRole(r.Role),
            CreatedAt: toTime(r.CreatedAt),
        })
    }
    return res, nil
}

// ---------------- Invites ----------------

func (p *pgRepo) CreateInvite(ctx context.Context, orgID uuid.UUID, inviterID uuid.UUID, email string, role models.OrgRole, tokenHash string, expiresAt time.Time) error {
    const q = `INSERT INTO org_invites (token_hash, org_id, email, role, inviter_id, expires_at)
               VALUES ($1, $2, $3, $4::org_role, $5, $6)`
    _, err := p.q.Exec(ctx, q, tokenHash, fromUUID(orgID), email, string(role), fromUUID(inviterID), expiresAt)
    if err != nil {
        slog.ErrorContext(ctx, "CreateInvite failed", "err", err)
    }
    return err
}

func (p *pgRepo) GetInviteByTokenHash(ctx context.Context, tokenHash string) (models.OrgInvite, error) {
    const q = `SELECT token_hash, org_id, email, role::text, inviter_id, expires_at, used_at
               FROM org_invites WHERE token_hash = $1`
    row := p.q.QueryRow(ctx, q, tokenHash)
    var m models.OrgInvite
    var role string
    var usedAt pgtype.Timestamptz
    var orgID pgtype.UUID
    var inviterID pgtype.UUID
    if err := row.Scan(&m.TokenHash, &orgID, &m.Email, &role, &inviterID, &m.ExpiresAt, &usedAt); err != nil {
        slog.ErrorContext(ctx, "GetInviteByTokenHash failed", "err", err)
        return models.OrgInvite{}, err
    }
    m.OrgID = toUUID(orgID)
    m.InviterID = toUUID(inviterID)
    m.Role = models.OrgRole(role)
    if usedAt.Valid {
        m.UsedAt = usedAt.Time
    }
    return m, nil
}

func (p *pgRepo) UseInvite(ctx context.Context, tokenHash string) error {
    const q = `UPDATE org_invites SET used_at = now() WHERE token_hash = $1 AND used_at IS NULL`
    _, err := p.q.Exec(ctx, q, tokenHash)
    if err != nil {
        slog.ErrorContext(ctx, "UseInvite failed", "err", err)
    }
    return err
}
