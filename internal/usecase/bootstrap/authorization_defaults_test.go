package bootstrap

import (
	"context"
	"testing"
	"time"

	domainAuthorization "github.com/freeDog-wy/go-backend-template/internal/domain/authorization"
)

func TestInitializeAuthorizationDefaultsInstallsRoleAndPermissions(t *testing.T) {
	t.Parallel()

	repo := &authorizationDefaultsRepositoryFake{}
	if err := initializeAuthorizationDefaults(context.Background(), repo); err != nil {
		t.Fatalf("initializeAuthorizationDefaults() error = %v", err)
	}

	wantPermissions := domainAuthorization.DefaultPermissions()
	if len(repo.savedPermissionCodes) != len(wantPermissions) {
		t.Fatalf("saved permissions = %d, want %d", len(repo.savedPermissionCodes), len(wantPermissions))
	}
	for index, permission := range wantPermissions {
		if repo.savedPermissionCodes[index] != permission.GetCode() {
			t.Fatalf("permission code at %d = %q, want %q", index, repo.savedPermissionCodes[index], permission.GetCode())
		}
	}
	if repo.savedRoleCode != domainAuthorization.SuperAdminRoleCode {
		t.Fatalf("role code = %q, want %q", repo.savedRoleCode, domainAuthorization.SuperAdminRoleCode)
	}
	if len(repo.boundPermissionIDs) != len(wantPermissions) {
		t.Fatalf("bound permissions = %v, want %d IDs", repo.boundPermissionIDs, len(wantPermissions))
	}
	for index, permissionID := range repo.boundPermissionIDs {
		if permissionID != uint(index+1) {
			t.Fatalf("permission ID at %d = %d, want %d", index, permissionID, index+1)
		}
	}
}

type authorizationDefaultsRepositoryFake struct {
	savedPermissionCodes []string
	savedRoleCode        string
	boundPermissionIDs   []uint
}

func (r *authorizationDefaultsRepositoryFake) EnsurePermissions(_ context.Context, permissions []*domainAuthorization.Permission) error {
	r.savedPermissionCodes = make([]string, 0, len(permissions))
	for _, permission := range permissions {
		r.savedPermissionCodes = append(r.savedPermissionCodes, permission.GetCode())
	}
	return nil
}

func (r *authorizationDefaultsRepositoryFake) EnsureRole(_ context.Context, role *domainAuthorization.Role) (*domainAuthorization.Role, error) {
	r.savedRoleCode = role.GetCode()
	return domainAuthorization.ReconstituteRole(100, role.GetCode(), role.GetName(), role.GetDescription(), time.Now(), time.Now()), nil
}

func (*authorizationDefaultsRepositoryFake) FindPermissionsByCodes(_ context.Context, codes []string) ([]*domainAuthorization.Permission, error) {
	permissions := make([]*domainAuthorization.Permission, 0, len(codes))
	for index, code := range codes {
		permissions = append(permissions, domainAuthorization.ReconstitutePermission(uint(index+1), code, code, "", time.Now(), time.Now()))
	}
	return permissions, nil
}

func (r *authorizationDefaultsRepositoryFake) EnsureRolePermissions(_ context.Context, _ uint, permissionIDs []uint) error {
	r.boundPermissionIDs = append([]uint(nil), permissionIDs...)
	return nil
}
