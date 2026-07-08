package identity

import "github.com/freeDog-wy/go-backend-template/internal/domain/shared"

// RegisterCmd 注册命令——与 HTTP 协议无关。
type RegisterCmd struct {
	Name        string
	Email       string
	Password    string
	CaptchaID   string
	CaptchaCode string
}

type CreateAdminUserCmd struct {
	Name     string
	Email    string
	Password string
	RoleIDs  []uint
}

type UpdateProfileCmd struct {
	UserID uint
	Name   string
}

type UpdateStatusCmd struct {
	UserID      uint
	Status      string
	ActorUserID uint
	IP          string
	UserAgent   string
}

type ListUsersCmd struct {
	Page shared.PageQuery
}
