package authorization

import (
	"context"

	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
)

// Repository 定义角色、权限及其关系的持久化契约。
//
// Ensure 前缀的方法必须可重复执行，以支持部署和首次管理员操作期间的并发初始化。
// Replace 前缀的方法以完整输入替换关系集合，调用方负责在事务内完成关联修改。
type Repository interface {
	// EnsurePermissions 确保权限码存在，不应删除或重置已有权限。
	EnsurePermissions(ctx context.Context, permissions []*Permission) error
	// EnsureRole 确保角色存在，并返回当前持久化状态。
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
	EnsureRolePermissions(ctx context.Context, roleID uint, permissionIDs []uint) error
	// ReplaceRolePermissions 以完整输入替换角色的权限集合。
	ReplaceRolePermissions(ctx context.Context, roleID uint, permissionIDs []uint) error

	ListUserRoles(ctx context.Context, userID uint) ([]*Role, error)
	ListUserPermissionCodes(ctx context.Context, userID uint) ([]string, error)
	// ReplaceUserRoles 以完整输入替换用户的角色集合。
	ReplaceUserRoles(ctx context.Context, userID uint, roleIDs []uint) error
	CountUsersByRoleCode(ctx context.Context, roleCode string) (int64, error)
}
