//go:build integration

package authorization

import (
	"context"
	"fmt"
	"testing"
	"time"

	domainAuthorization "github.com/freeDog-wy/go-backend-template/internal/domain/authorization"
	modelAuthorization "github.com/freeDog-wy/go-backend-template/internal/model/authorization"
	"github.com/freeDog-wy/go-backend-template/internal/testsupport"
)

func TestRepositoryIntegrationRoleBindings(t *testing.T) {
	db := testsupport.OpenPostgres(t)
	if err := db.AutoMigrate(&modelAuthorization.Role{}, &modelAuthorization.Permission{}, &modelAuthorization.UserRole{}, &modelAuthorization.RolePermission{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	repo := New(db)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	role, _ := domainAuthorization.NewRole("operator-"+suffix, "Operator", "")
	permission, _ := domainAuthorization.NewPermission("report.read."+suffix, "Read reports", "")
	if err := repo.CreateRole(context.Background(), role); err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	if err := repo.EnsurePermissions(context.Background(), []*domainAuthorization.Permission{permission}); err != nil {
		t.Fatalf("EnsurePermissions() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Where("role_id = ?", role.GetID()).Delete(&modelAuthorization.RolePermission{}).Error
		_ = db.Where("user_id = ?", uint(99)).Delete(&modelAuthorization.UserRole{}).Error
		_ = db.Where("id = ?", role.GetID()).Delete(&modelAuthorization.Role{}).Error
		_ = db.Where("code = ?", permission.GetCode()).Delete(&modelAuthorization.Permission{}).Error
	})
	permissions, err := repo.FindPermissionsByCodes(context.Background(), []string{permission.GetCode()})
	if err != nil || len(permissions) != 1 {
		t.Fatalf("FindPermissionsByCodes() = %#v, %v", permissions, err)
	}
	if err := repo.EnsureRolePermissions(context.Background(), role.GetID(), []uint{permissions[0].GetID()}); err != nil {
		t.Fatalf("EnsureRolePermissions() error = %v", err)
	}
	if err := repo.EnsureRolePermissions(context.Background(), role.GetID(), []uint{permissions[0].GetID()}); err != nil {
		t.Fatalf("EnsureRolePermissions() second call error = %v", err)
	}
	if err := repo.ReplaceUserRoles(context.Background(), 99, []uint{role.GetID()}); err != nil {
		t.Fatalf("ReplaceUserRoles() error = %v", err)
	}
	codes, err := repo.ListUserPermissionCodes(context.Background(), 99)
	if err != nil || len(codes) != 1 || codes[0] != permission.GetCode() {
		t.Fatalf("ListUserPermissionCodes() = %v, %v", codes, err)
	}
	if err := repo.ReplaceUserRoles(context.Background(), 99, nil); err != nil {
		t.Fatalf("clear user roles: %v", err)
	}
	codes, err = repo.ListUserPermissionCodes(context.Background(), 99)
	if err != nil || len(codes) != 0 {
		t.Fatalf("codes after clear = %v, %v", codes, err)
	}
}
