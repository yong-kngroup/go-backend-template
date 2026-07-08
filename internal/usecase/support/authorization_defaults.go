package support

import (
	"context"

	domainAuthorization "github.com/freeDog-wy/go-backend-template/internal/domain/authorization"
)

type AuthorizationDefaultsInstaller struct {
	repo domainAuthorization.Repository
}

func NewAuthorizationDefaultsInstaller(repo domainAuthorization.Repository) *AuthorizationDefaultsInstaller {
	return &AuthorizationDefaultsInstaller{repo: repo}
}

func (i *AuthorizationDefaultsInstaller) Ensure(ctx context.Context) error {
	permissions := domainAuthorization.DefaultPermissions()
	if err := i.repo.EnsurePermissions(ctx, permissions); err != nil {
		return err
	}

	superAdmin, err := domainAuthorization.NewSuperAdminRole()
	if err != nil {
		return err
	}
	role, err := i.repo.EnsureRole(ctx, superAdmin)
	if err != nil {
		return err
	}

	savedPermissions, err := i.repo.FindPermissionsByCodes(ctx, domainAuthorization.PermissionCodes(permissions))
	if err != nil {
		return err
	}

	permissionIDs := make([]uint, 0, len(savedPermissions))
	for _, permission := range savedPermissions {
		permissionIDs = append(permissionIDs, permission.GetID())
	}
	return i.repo.ReplaceRolePermissions(ctx, role.GetID(), permissionIDs)
}
