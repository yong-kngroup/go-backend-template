package authorization

import (
	"context"

	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
)

type Repository interface {
	EnsurePermissions(ctx context.Context, permissions []*Permission) error
	EnsureRole(ctx context.Context, role *Role) (*Role, error)
	ListRoles(ctx context.Context, page shared.PageQuery) ([]*Role, int64, error)
	FindRoleByID(ctx context.Context, roleID uint) (*Role, error)
	FindRoleByCode(ctx context.Context, code string) (*Role, error)
	FindRolesByIDs(ctx context.Context, roleIDs []uint) ([]*Role, error)
	CreateRole(ctx context.Context, role *Role) error
	UpdateRole(ctx context.Context, role *Role) error

	ListPermissions(ctx context.Context, page shared.PageQuery) ([]*Permission, int64, error)
	FindPermissionsByCodes(ctx context.Context, codes []string) ([]*Permission, error)
	ListRolePermissions(ctx context.Context, roleID uint) ([]*Permission, error)
	ReplaceRolePermissions(ctx context.Context, roleID uint, permissionIDs []uint) error

	ListUserRoles(ctx context.Context, userID uint) ([]*Role, error)
	ListUserPermissionCodes(ctx context.Context, userID uint) ([]string, error)
	ReplaceUserRoles(ctx context.Context, userID uint, roleIDs []uint) error
	CountUsersByRoleCode(ctx context.Context, roleCode string) (int64, error)
}
