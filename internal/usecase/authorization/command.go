package authorization

import "github.com/freeDog-wy/go-backend-template/internal/domain/shared"

type CreateRoleCmd struct {
	Code            string
	Name            string
	Description     string
	PermissionCodes []string
}

type UpdateRoleCmd struct {
	RoleID          uint
	Name            string
	Description     string
	PermissionCodes []string
}

type ReplaceUserRolesCmd struct {
	UserID      uint
	RoleIDs     []uint
	ActorUserID uint
	IP          string
	UserAgent   string
}

type AdminLoginCmd struct {
	UserID uint
}

type ListRolesCmd struct {
	Page shared.PageQuery
}

type ListPermissionsCmd struct {
	Page shared.PageQuery
}
