package mcp

import (
	"context"
	"testing"
	"time"

	domainAuthorization "github.com/freeDog-wy/go-backend-template/internal/domain/authorization"
	domainIdentity "github.com/freeDog-wy/go-backend-template/internal/domain/identity"
	domainMCP "github.com/freeDog-wy/go-backend-template/internal/domain/mcp"
	domainShared "github.com/freeDog-wy/go-backend-template/internal/domain/shared"
)

func TestBootstrapUsesEnsureRolePermissions(t *testing.T) {
	t.Parallel()

	user, err := domainIdentity.NewAdminUser("MCP Operator", "mcp@example.com")
	if err != nil {
		t.Fatalf("NewAdminUser() error = %v", err)
	}
	user.AssignID(7)

	repo := &bootstrapAuthorizationRepo{}
	for i, code := range servicePermissionCodes {
		permission, permissionErr := domainAuthorization.NewPermission(code, code, "")
		if permissionErr != nil {
			t.Fatalf("NewPermission(%q) error = %v", code, permissionErr)
		}
		repo.permissions = append(repo.permissions, domainAuthorization.ReconstitutePermission(
			uint(i+1),
			permission.GetCode(),
			permission.GetName(),
			permission.GetDescription(),
			time.Now(),
			time.Now(),
		))
	}

	service := NewBootstrapService(
		&bootstrapTx{},
		&bootstrapAccountRepo{},
		&bootstrapUserRepo{user: user},
		repo,
		&bootstrapHasher{},
		nil,
		nil,
	)

	err = service.Bootstrap(context.Background(), BootstrapCmd{
		Enabled:               true,
		Name:                  "MCP Operator",
		Email:                 "mcp@example.com",
		ClientID:              "client-id",
		ClientSecret:          "01234567890123456789012345678901",
		RotationGrace:         time.Minute,
		ServiceAccountEnabled: true,
	})
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}

	if repo.ensureRolePermissionsCalls != 1 {
		t.Fatalf("EnsureRolePermissions() calls = %d, want 1", repo.ensureRolePermissionsCalls)
	}
	if repo.replaceRolePermissionsCalls != 0 {
		t.Fatalf("ReplaceRolePermissions() calls = %d, want 0", repo.replaceRolePermissionsCalls)
	}
	if len(repo.replacedUserRoleIDs) != 1 || repo.replacedUserRoleIDs[0] != 1 {
		t.Fatalf("ReplaceUserRoles() role IDs = %v, want [1]", repo.replacedUserRoleIDs)
	}
}

type bootstrapTx struct{}

func (*bootstrapTx) Do(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type bootstrapAccountRepo struct{}

func (*bootstrapAccountRepo) Create(context.Context, *domainMCP.ServiceAccount) error { return nil }
func (*bootstrapAccountRepo) Update(context.Context, *domainMCP.ServiceAccount) error { return nil }
func (*bootstrapAccountRepo) FindByClientID(context.Context, string) (*domainMCP.ServiceAccount, error) {
	return nil, domainShared.ErrNotFound
}
func (*bootstrapAccountRepo) FindByUserID(context.Context, uint) (*domainMCP.ServiceAccount, error) {
	return nil, domainShared.ErrNotFound
}

type bootstrapUserRepo struct {
	user *domainIdentity.User
}

func (r *bootstrapUserRepo) FindByID(context.Context, uint) (*domainIdentity.User, error) {
	return nil, domainShared.ErrNotFound
}
func (r *bootstrapUserRepo) FindByEmail(context.Context, string) (*domainIdentity.User, error) {
	if r.user == nil {
		return nil, domainShared.ErrNotFound
	}
	return r.user, nil
}
func (*bootstrapUserRepo) List(context.Context, domainShared.PageQuery) ([]*domainIdentity.User, int64, error) {
	return nil, 0, nil
}
func (*bootstrapUserRepo) Create(context.Context, *domainIdentity.User) error { return nil }
func (*bootstrapUserRepo) Update(context.Context, *domainIdentity.User) error { return nil }
func (*bootstrapUserRepo) Delete(context.Context, uint) error                 { return nil }

type bootstrapAuthorizationRepo struct {
	permissions                 []*domainAuthorization.Permission
	ensureRolePermissionsCalls  int
	replaceRolePermissionsCalls int
	replacedUserRoleIDs         []uint
}

func (*bootstrapAuthorizationRepo) EnsurePermissions(context.Context, []*domainAuthorization.Permission) error {
	return nil
}
func (*bootstrapAuthorizationRepo) EnsureRole(context.Context, *domainAuthorization.Role) (*domainAuthorization.Role, error) {
	return domainAuthorization.ReconstituteRole(1, serviceRoleCode, "CMS MCP Operator", "", time.Now(), time.Now()), nil
}
func (*bootstrapAuthorizationRepo) ListRoles(context.Context, domainShared.PageQuery) ([]*domainAuthorization.Role, int64, error) {
	return nil, 0, nil
}
func (*bootstrapAuthorizationRepo) FindRoleByID(context.Context, uint) (*domainAuthorization.Role, error) {
	return nil, domainShared.ErrNotFound
}
func (*bootstrapAuthorizationRepo) FindRoleByCode(context.Context, string) (*domainAuthorization.Role, error) {
	return nil, domainShared.ErrNotFound
}
func (*bootstrapAuthorizationRepo) FindRolesByIDs(context.Context, []uint) ([]*domainAuthorization.Role, error) {
	return nil, nil
}
func (*bootstrapAuthorizationRepo) CreateRole(context.Context, *domainAuthorization.Role) error {
	return nil
}
func (*bootstrapAuthorizationRepo) UpdateRole(context.Context, *domainAuthorization.Role) error {
	return nil
}
func (*bootstrapAuthorizationRepo) ListPermissions(context.Context, domainShared.PageQuery) ([]*domainAuthorization.Permission, int64, error) {
	return nil, 0, nil
}
func (r *bootstrapAuthorizationRepo) FindPermissionsByCodes(context.Context, []string) ([]*domainAuthorization.Permission, error) {
	return r.permissions, nil
}
func (*bootstrapAuthorizationRepo) ListRolePermissions(context.Context, uint) ([]*domainAuthorization.Permission, error) {
	return nil, nil
}
func (r *bootstrapAuthorizationRepo) EnsureRolePermissions(context.Context, uint, []uint) error {
	r.ensureRolePermissionsCalls++
	return nil
}
func (r *bootstrapAuthorizationRepo) ReplaceRolePermissions(context.Context, uint, []uint) error {
	r.replaceRolePermissionsCalls++
	return nil
}
func (*bootstrapAuthorizationRepo) ListUserRoles(context.Context, uint) ([]*domainAuthorization.Role, error) {
	return nil, nil
}
func (*bootstrapAuthorizationRepo) ListUserPermissionCodes(context.Context, uint) ([]string, error) {
	return nil, nil
}
func (r *bootstrapAuthorizationRepo) ReplaceUserRoles(_ context.Context, _ uint, roleIDs []uint) error {
	r.replacedUserRoleIDs = append([]uint(nil), roleIDs...)
	return nil
}
func (*bootstrapAuthorizationRepo) CountUsersByRoleCode(context.Context, string) (int64, error) {
	return 0, nil
}

type bootstrapHasher struct{}

func (*bootstrapHasher) Hash(string) (string, error) { return "hash", nil }
func (*bootstrapHasher) Verify(string, string) bool  { return false }

var (
	_ domainMCP.ServiceAccountRepository = (*bootstrapAccountRepo)(nil)
	_ domainIdentity.Repository          = (*bootstrapUserRepo)(nil)
	_ domainAuthorization.Repository     = (*bootstrapAuthorizationRepo)(nil)
	_ domainShared.TxManager             = (*bootstrapTx)(nil)
)
