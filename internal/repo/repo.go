// internal/repo/repo.go
package repo

import (
	"context"
	"net/netip"
	"time"

	"github.com/google/uuid"

	db "yourapp/internal/db/gen" // <-- change if your sqlc package path differs
	"yourapp/internal/models"
)

// Repo defines the methods the rest of the app uses.
type Repo interface {
	UpsertUserByVerifiedEmail(ctx context.Context, email, name string) (models.User, error)
	LinkIdentity(ctx context.Context, userID uuid.UUID, provider, subject string) error
	GetUserByIdentity(ctx context.Context, provider, subject string) (models.User, error)
	GetUserByEmail(ctx context.Context, email string) (models.User, error)

	FindOrgBySlug(ctx context.Context, slug string) (models.Org, error)
	FindOrgByID(ctx context.Context, id uuid.UUID) (models.Org, error)
	FindOrgByTenantID(ctx context.Context, tid string) (models.Org, error)
	CreateOrg(ctx context.Context, slug, name, tenantID string) (models.Org, error)
	EnsureMembership(ctx context.Context, orgID, userID uuid.UUID, defaultRole models.OrgRole) (models.OrgRole, error)
	GetRole(ctx context.Context, orgID, userID uuid.UUID) (models.OrgRole, error)
	GetUserWithOrgAndRole(ctx context.Context, uid, oid uuid.UUID) (models.User, models.Org, models.OrgRole, error)
	ListIdentitiesForUser(ctx context.Context, uid uuid.UUID) ([]models.LinkedIdentity, error)
	ListUserOrgs(ctx context.Context, uid uuid.UUID) ([]models.OrgSummary, error)
	GetLastSuccessfulLoginByUsername(ctx context.Context, username string) (time.Time, bool)
	ApplyGroupRoleMappings(ctx context.Context, orgID uuid.UUID, provider string, groupIDs []string) (models.OrgRole, error)

	// Local auth
	CreateLocalCredential(ctx context.Context, uid uuid.UUID, username, phc string) error
	GetLocalCredentialByUsername(ctx context.Context, username string) (models.LocalCredential, models.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (models.User, error)
	PickUserOrg(ctx context.Context, uid uuid.UUID) (models.Org, error)
	SearchUsers(ctx context.Context, org_id uuid.UUID, payload []byte) ([]models.User, error)

	// Generic EAV table search
	SearchUserTable(ctx context.Context, org_id uuid.UUID, table string, payload []byte) ([]models.TableRow, error)
	GetUserTableSchema(ctx context.Context, org_id uuid.UUID, table string) ([]models.TableColumn, error)

	// User-defined tables (org-scoped)
	CreateUserTable(ctx context.Context, orgID uuid.UUID, name string) (models.UserTable, bool, error)
	ListUserTables(ctx context.Context, orgID uuid.UUID) ([]models.UserTable, error)
	DeleteUserTable(ctx context.Context, orgID uuid.UUID, table string) (models.UserTable, bool, error)

    // Row lookup by UUID (org-scoped)
    GetRowData(ctx context.Context, orgID uuid.UUID, rowID uuid.UUID) (map[string]any, bool, error)

    // Lookup minimal rows by indexed column (for exposing UUIDs)
    LookupIndexedRows(ctx context.Context, orgID uuid.UUID, table string, field *string, q *string, limit int) ([]models.IndexedRow, error)

    // List indexed fields (text/enum) for cross-table reference building
    ListIndexedFields(ctx context.Context, orgID uuid.UUID) ([]models.IndexedField, error)

    // Delete a row by UUID from a table within the org
    DeleteUserTableRow(ctx context.Context, orgID uuid.UUID, table string, rowID uuid.UUID) (bool, error)

    // Resolve a human label for a referenced row id in a given table
    GetRowLabel(ctx context.Context, orgID uuid.UUID, tableID int64, rowID uuid.UUID) (string, error)
    // Resolve a human label for a row id by inspecting its table (org-scoped)
    GetRowLabelAuto(ctx context.Context, orgID uuid.UUID, rowID uuid.UUID) (string, error)

	// Columns management
	AddUserTableColumn(ctx context.Context, orgID uuid.UUID, table string, input models.TableColumnInput) (models.TableColumn, bool, error)
	RemoveUserTableColumn(ctx context.Context, orgID uuid.UUID, table string, columnName string) (models.TableColumn, bool, error)

	// Rows management
	InsertUserTableRow(ctx context.Context, orgID uuid.UUID, table string, values []byte) (models.TableRow, error)

	UserHasTOTP(ctx context.Context, uid uuid.UUID) bool
	SetTOTPSecret(ctx context.Context, uid uuid.UUID, secret, issuer, label string) error
	GetTOTPSecret(ctx context.Context, uid uuid.UUID) (string, bool)

	// User profile updates
	UpdateUserProfile(ctx context.Context, userID uuid.UUID, name *string, avatarURL *string, phone *string, country *string) error

	// Login events
	RecordLoginSuccess(ctx context.Context, username string, ip netip.Addr) error
	RecordLoginFailure(ctx context.Context, username string, ip netip.Addr) error

	// Invites
	CreateInvite(ctx context.Context, orgID uuid.UUID, inviterID uuid.UUID, email string, role models.OrgRole, tokenHash string, expiresAt time.Time) error
	GetInviteByTokenHash(ctx context.Context, tokenHash string) (models.OrgInvite, error)
	UseInvite(ctx context.Context, tokenHash string) error

	// Local credential management
	UpdateLocalPasswordHash(ctx context.Context, userID uuid.UUID, phc string) error
}

// pgRepo wraps the sqlc Queries.
type pgRepo struct{ q *db.Queries }

func New(q *db.Queries) Repo { return &pgRepo{q: q} }
