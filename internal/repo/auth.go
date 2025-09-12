package repo

import (
    "context"
    "log/slog"
    "strings"
    "net/netip"

    "github.com/google/uuid"

    db "yourapp/internal/db/gen"
    "yourapp/internal/models"
)

// ---------------- Local credentials & TOTP ----------------

func (p *pgRepo) CreateLocalCredential(ctx context.Context, uid uuid.UUID, username, phc string) error {
    slog.DebugContext(ctx, "CreateLocalCredential", "user_id", uid.String(), "username", strings.ToLower(username))
    return p.q.CreateLocalCredential(ctx, db.CreateLocalCredentialParams{
        UserID:       fromUUID(uid),
        Lower:        strings.ToLower(username),
        PasswordHash: phc,
    })
}

func (p *pgRepo) GetLocalCredentialByUsername(ctx context.Context, username string) (models.LocalCredential, models.User, error) {
    slog.DebugContext(ctx, "GetLocalCredentialByUsername", "username", strings.ToLower(username))
    row, err := p.q.GetLocalCredentialByUsername(ctx, username)
    if err != nil {
        slog.ErrorContext(ctx, "GetLocalCredentialByUsername failed", "err", err)
        return models.LocalCredential{}, models.User{}, err
    }
    lc := models.LocalCredential{
        UserID:       toUUID(row.UserID),
        Username:     row.Username,
        PasswordHash: row.PasswordHash,
    }
    u := models.User{
        ID:    toUUID(row.UserID),
        Email: row.Email,
        Name:  fromText(row.Name),
    }
    return lc, u, nil
}

func (p *pgRepo) UserHasTOTP(ctx context.Context, uid uuid.UUID) bool {
    slog.DebugContext(ctx, "UserHasTOTP", "user_id", uid.String())
    ok, err := p.q.UserHasTOTP(ctx, fromUUID(uid))
    if err != nil {
        slog.ErrorContext(ctx, "UserHasTOTP failed", "err", err)
        return false
    }
    return ok
}

func (p *pgRepo) SetTOTPSecret(ctx context.Context, uid uuid.UUID, secret, issuer, label string) error {
    slog.DebugContext(ctx, "SetTOTPSecret", "user_id", uid.String(), "issuer", issuer, "label", label)
    return p.q.SetTOTPSecret(ctx, db.SetTOTPSecretParams{
        UserID: fromUUID(uid),
        Secret: secret,
        Issuer: issuer,
        Label:  label,
    })
}

func (p *pgRepo) GetTOTPSecret(ctx context.Context, uid uuid.UUID) (string, bool) {
    slog.DebugContext(ctx, "GetTOTPSecret", "user_id", uid.String())
    sec, err := p.q.GetTOTPSecret(ctx, fromUUID(uid))
    if err != nil {
        slog.ErrorContext(ctx, "GetTOTPSecret failed", "err", err)
        return "", false
    }
    return sec, true
}

func (p *pgRepo) UpdateLocalPasswordHash(ctx context.Context, uid uuid.UUID, phc string) error {
    slog.DebugContext(ctx, "UpdateLocalPasswordHash", "user_id", uid.String())
    return p.q.UpdateLocalPasswordHash(ctx, db.UpdateLocalPasswordHashParams{
        UserID:       fromUUID(uid),
        PasswordHash: phc,
    })
}

// -------- Login attempt recording --------

func (p *pgRepo) RecordLoginSuccess(ctx context.Context, username string, ip netip.Addr) error {
    slog.DebugContext(ctx, "RecordLoginSuccess", "username", strings.ToLower(username), "ip", ip.String())
    return p.q.RecordLoginAttempt(ctx, db.RecordLoginAttemptParams{
        Lower:   strings.ToLower(username),
        Ip:      ip,
        Success: true,
    })
}

func (p *pgRepo) RecordLoginFailure(ctx context.Context, username string, ip netip.Addr) error {
    slog.DebugContext(ctx, "RecordLoginFailure", "username", strings.ToLower(username), "ip", ip.String())
    return p.q.RecordLoginAttempt(ctx, db.RecordLoginAttemptParams{
        Lower:   strings.ToLower(username),
        Ip:      ip,
        Success: false,
    })
}
