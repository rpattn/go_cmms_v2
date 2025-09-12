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

// ---------------- Users & Identities ----------------

func (p *pgRepo) UpsertUserByVerifiedEmail(ctx context.Context, email string, name string) (models.User, error) {
	slog.DebugContext(ctx, "UpsertUserByVerifiedEmail", "email", email)
	u, err := p.q.UpsertUserByVerifiedEmail(ctx, db.UpsertUserByVerifiedEmailParams{
		Email: email,
		Name:  toText(name),
	})
	if err != nil {
		slog.ErrorContext(ctx, "UpsertUserByVerifiedEmail failed", "err", err)
		return models.User{}, err
	}
    return models.User{
        ID:    toUUID(u.ID),
        Email: u.Email,
        Name:  fromText(u.Name),
        AvatarURL: fromText(u.AvatarUrl),
        Phone: fromText(u.Phone),
        Country: fromText(u.Country),
    }, nil
}

func (p *pgRepo) LinkIdentity(ctx context.Context, userID uuid.UUID, provider, subject string) error {
	slog.DebugContext(ctx, "LinkIdentity", "user_id", userID.String(), "provider", provider)
	return p.q.LinkIdentity(ctx, db.LinkIdentityParams{
		UserID:   fromUUID(userID),
		Provider: provider,
		Subject:  subject,
	})
}

func (p *pgRepo) GetUserByIdentity(ctx context.Context, provider, subject string) (models.User, error) {
	slog.DebugContext(ctx, "GetUserByIdentity", "provider", provider)
	row, err := p.q.GetUserByIdentity(ctx, db.GetUserByIdentityParams{Provider: provider, Subject: subject})
	if err != nil {
		slog.ErrorContext(ctx, "GetUserByIdentity failed", "err", err)
		return models.User{}, err
	}
	return models.User{
		ID:    toUUID(row.ID),
		Email: row.Email,
		Name:  fromText(row.Name),
	}, nil
}

func (p *pgRepo) GetUserByEmail(ctx context.Context, email string) (models.User, error) {
	slog.DebugContext(ctx, "GetUserByEmail", "email", email)
	row, err := p.q.GetUserByEmail(ctx, email)
	if err != nil {
		slog.ErrorContext(ctx, "GetUserByEmail failed", "err", err)
		return models.User{}, err
	}
	return models.User{
		ID:    toUUID(row.ID),
		Email: row.Email,
		Name:  fromText(row.Name),
		AvatarURL: fromText(row.AvatarUrl),
		Phone: fromText(row.Phone),
		Country: fromText(row.Country),
	}, nil
}

func (p *pgRepo) UpdateUserProfile(ctx context.Context, userID uuid.UUID, name *string, avatarURL *string, phone *string, country *string) error {
	params := db.UpdateUserProfileParams{
		UserID:    fromUUID(userID),
		Name:      toNullText(name),
		AvatarUrl: toNullText(avatarURL),
		Phone:     toNullText(phone),
		Country:   toNullText(country),
	}
	return p.q.UpdateUserProfile(ctx, params)
}

func (p *pgRepo) ListIdentitiesForUser(ctx context.Context, uid uuid.UUID) ([]models.LinkedIdentity, error) {
    rows, err := p.q.ListIdentitiesForUser(ctx, fromUUID(uid))
    if err != nil {
        return nil, err
    }
    out := make([]models.LinkedIdentity, 0, len(rows))
    for _, r := range rows {
        out = append(out, models.LinkedIdentity{Provider: r.Provider, Subject: r.Subject})
    }
    return out, nil
}

func (p *pgRepo) GetLastSuccessfulLoginByUsername(ctx context.Context, username string) (time.Time, bool) {
    ts, err := p.q.GetLastSuccessfulLoginByUsername(ctx, username)
    if err != nil {
        return time.Time{}, false
    }
    if !ts.Valid {
        return time.Time{}, false
    }
    return ts.Time, true
}

// GetUserByUUID fetches a user by their UUID.
func (p *pgRepo) GetUserByID(ctx context.Context, id uuid.UUID) (models.User, error) {
	slog.DebugContext(ctx, "GetUserByID", "user_id", id.String())
	row, err := p.q.GetUserByID(ctx, toPgUUID(id))
	if err != nil {
		slog.ErrorContext(ctx, "GetUserByID failed", "err", err)
		return models.User{}, err
	}
    u := models.User{
        ID:    toUUID(row.ID),
        Email: row.Email,
        Name:  fromText(row.Name),
        AvatarURL: fromText(row.AvatarUrl),
        Phone: fromText(row.Phone),
        Country: fromText(row.Country),
    }
	return u, nil
}

func (p *pgRepo) GetUserWithOrgAndRole(
	ctx context.Context,
	uid uuid.UUID,
	oid uuid.UUID,
) (models.User, models.Org, models.OrgRole, error) {
	slog.DebugContext(ctx, "GetUserWithOrgAndRole", "user_id", uid.String(), "org_id", oid.String())
	params := db.GetUserWithOrgAndRoleParams{
		Column1: toPgUUID(uid),
		Column2: toPgUUID(oid),
	}

	row, err := p.q.GetUserWithOrgAndRole(ctx, params)
	if err != nil {
		slog.ErrorContext(ctx, "GetUserWithOrgAndRole failed", "err", err)
		return models.User{}, models.Org{}, "", err
	}

	toBool := func(x interface{}) bool {
		switch v := x.(type) {
		case bool:
			return v
		case pgtype.Bool:
			return v.Bool
		case string:
			return v == "t" || v == "true" || v == "1"
		case []byte:
			s := string(v)
			return s == "t" || s == "true" || s == "1"
		case int64:
			return v != 0
		case nil:
			return false
		default:
			return false
		}
	}
	toString := func(x interface{}) (string, bool) {
		switch v := x.(type) {
		case string:
			return v, v != ""
		case []byte:
			if len(v) == 0 {
				return "", false
			}
			return string(v), true
		case pgtype.Text:
			if v.Valid {
				return v.String, true
			}
			return "", false
		case nil:
			return "", false
		default:
			return fmt.Sprintf("%v", v), true
		}
	}

	if !toBool(row.UserExists) {
		return models.User{}, models.Org{}, "", models.ErrUserNotFound
	}
	if !toBool(row.OrgExists) {
		return models.User{}, models.Org{}, "", models.ErrOrgNotFound
	}
	if !toBool(row.RoleExists) {
		return models.User{}, models.Org{}, "", models.ErrRoleNotFound
	}

	var (
		uID = row.UserID.Bytes
		oID = row.OrgID.Bytes
	)
	u := models.User{
		ID:    uID,
		Email: textOrEmpty(row.UserEmail),
		Name:  textOrEmpty(row.UserName),
		AvatarURL: textOrEmpty(row.UserAvatarUrl),
		Phone:     textOrEmpty(row.UserPhone),
		Country:   textOrEmpty(row.UserCountry),
	}
	o := models.Org{
		ID:       oID,
		Slug:     textOrEmpty(row.OrgSlug),
		Name:     textOrEmpty(row.OrgName),
		TenantID: "",
	}

	roleStr, _ := toString(row.Role)
	role := models.OrgRole(roleStr)

	return u, o, role, nil
}
