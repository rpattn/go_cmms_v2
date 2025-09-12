// internal/models/types.go
package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// WorkOrder is the domain-level struct returned by repositories.
type WorkOrder struct {
	ID          uuid.UUID `json:"id"`
	OrgID       uuid.UUID `json:"org_id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	DueDate     time.Time `json:"due_date,omitempty"`
	CustomID    string    `json:"custom_id,omitempty"`
}

type OrgRole string

const (
	RoleOwner  OrgRole = "Owner"
	RoleAdmin  OrgRole = "Admin"
	RoleMember OrgRole = "Member"
	RoleViewer OrgRole = "Viewer"
)

type User struct {
    ID    uuid.UUID
    Email string
    Name  string
    AvatarURL string
    Phone string
    Country string
}

type LinkedIdentity struct {
    Provider string
    Subject  string
}

type OrgSummary struct {
    ID       uuid.UUID
    Slug     string
    Name     string
    Role     OrgRole
    CreatedAt time.Time
}

type Location struct {
    ID        uuid.UUID `json:"id"`
    Name      string    `json:"name"`
    CreatedAt time.Time `json:"created_at"`
}

type Team struct {
    ID        uuid.UUID `json:"id"`
    Name      string    `json:"name"`
    CreatedAt time.Time `json:"created_at"`
}

type Asset struct {
    ID        uuid.UUID `json:"id"`
    Name      string    `json:"name"`
    CreatedAt time.Time `json:"created_at"`
}

var (
	ErrUserNotFound = errors.New("user not found")
	ErrOrgNotFound  = errors.New("org not found")
	ErrRoleNotFound = errors.New("role not found")
)

type Org struct {
	ID       uuid.UUID
	Slug     string
	Name     string
	TenantID string
}

type LocalCredential struct {
	UserID       uuid.UUID
	Username     string
	PasswordHash string
}

type Session struct {
    UserID    uuid.UUID
    ActiveOrg uuid.UUID
    Provider  string
    Expiry    time.Time
}

// OrgInvite represents an invitation to join an organisation.
type OrgInvite struct {
    TokenHash string
    OrgID     uuid.UUID
    Email     string
    Role      OrgRole
    InviterID uuid.UUID
    ExpiresAt time.Time
    UsedAt    time.Time
}
