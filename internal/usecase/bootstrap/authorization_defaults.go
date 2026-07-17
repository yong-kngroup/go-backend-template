package bootstrap

import (
	"context"

	domainAuthorization "github.com/freeDog-wy/go-backend-template/internal/domain/authorization"
)

type authorizationDefaultsRepository interface {
	EnsurePermissions(ctx context.Context, permissions []*domainAuthorization.Permission) error
	EnsureRole(ctx context.Context, role *domainAuthorization.Role) (*domainAuthorization.Role, error)
	FindPermissionsByCodes(ctx context.Context, codes []string) ([]*domainAuthorization.Permission, error)
	EnsureRolePermissions(ctx context.Context, roleID uint, permissionIDs []uint) error
}

// initializeAuthorizationDefaults 安装系统预置权限、超级管理员角色及其权限关系。
// 它是启动初始化流程的一部分，调用方负责提供事务边界；Repository 的 Ensure 方法使
// 多实例重复启动仍保持幂等。
func initializeAuthorizationDefaults(ctx context.Context, repo authorizationDefaultsRepository) error {
	permissions := domainAuthorization.DefaultPermissions()
	if err := repo.EnsurePermissions(ctx, permissions); err != nil {
		return err
	}

	superAdmin, err := domainAuthorization.NewSuperAdminRole()
	if err != nil {
		return err
	}
	role, err := repo.EnsureRole(ctx, superAdmin)
	if err != nil {
		return err
	}

	savedPermissions, err := repo.FindPermissionsByCodes(ctx, domainAuthorization.PermissionCodes(permissions))
	if err != nil {
		return err
	}

	permissionIDs := make([]uint, 0, len(savedPermissions))
	for _, permission := range savedPermissions {
		permissionIDs = append(permissionIDs, permission.GetID())
	}
	return repo.EnsureRolePermissions(ctx, role.GetID(), permissionIDs)
}
