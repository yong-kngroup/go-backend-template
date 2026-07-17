//go:build integration

package bootstrap

import (
	"context"
	"testing"

	domainAuthorization "github.com/freeDog-wy/go-backend-template/internal/domain/authorization"
	"github.com/freeDog-wy/go-backend-template/internal/infra/database"
	modelAuthorization "github.com/freeDog-wy/go-backend-template/internal/model/authorization"
	repoAuthorization "github.com/freeDog-wy/go-backend-template/internal/repository/authorization"
	"github.com/freeDog-wy/go-backend-template/internal/testsupport"
	"gorm.io/gorm"
)

func TestInitializeAuthorizationIntegration(t *testing.T) {
	db := testsupport.OpenPostgres(t)
	if err := db.AutoMigrate(
		&modelAuthorization.Role{},
		&modelAuthorization.Permission{},
		&modelAuthorization.RolePermission{},
	); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	repo := repoAuthorization.New(db)
	service := New(database.NewTxManager(db), nil, repo, nil, nil, nil)
	ctx := context.Background()

	if err := service.InitializeAuthorization(ctx); err != nil {
		t.Fatalf("first InitializeAuthorization() error = %v", err)
	}
	if err := service.InitializeAuthorization(ctx); err != nil {
		t.Fatalf("second InitializeAuthorization() error = %v", err)
	}

	defaults := domainAuthorization.DefaultPermissions()
	role, err := repo.FindRoleByCode(ctx, domainAuthorization.SuperAdminRoleCode)
	if err != nil {
		t.Fatalf("FindRoleByCode() error = %v", err)
	}
	permissions, err := repo.ListRolePermissions(ctx, role.GetID())
	if err != nil {
		t.Fatalf("ListRolePermissions() error = %v", err)
	}
	if len(permissions) != len(defaults) {
		t.Fatalf("super admin permissions = %d, want %d", len(permissions), len(defaults))
	}

	wantCodes := make(map[string]struct{}, len(defaults))
	for _, permission := range defaults {
		wantCodes[permission.GetCode()] = struct{}{}
	}
	for _, permission := range permissions {
		if _, exists := wantCodes[permission.GetCode()]; !exists {
			t.Fatalf("unexpected super admin permission %q", permission.GetCode())
		}
	}

	assertAuthorizationDefaultsCount(t, db, len(defaults))
}

func assertAuthorizationDefaultsCount(t *testing.T, db *gorm.DB, permissionCount int) {
	t.Helper()

	var roles, permissions, bindings int64
	if err := db.Model(&modelAuthorization.Role{}).Count(&roles).Error; err != nil {
		t.Fatalf("count roles: %v", err)
	}
	if err := db.Model(&modelAuthorization.Permission{}).Count(&permissions).Error; err != nil {
		t.Fatalf("count permissions: %v", err)
	}
	if err := db.Model(&modelAuthorization.RolePermission{}).Count(&bindings).Error; err != nil {
		t.Fatalf("count role permissions: %v", err)
	}
	if roles != 1 || permissions != int64(permissionCount) || bindings != int64(permissionCount) {
		t.Fatalf("authorization defaults counts = roles:%d permissions:%d bindings:%d, want 1:%d:%d", roles, permissions, bindings, permissionCount, permissionCount)
	}
}
