package identity

import domainIdentity "github.com/freeDog-wy/go-backend-template/internal/domain/identity"

// UserResult 用户返回结果。
type UserResult struct {
	ID     uint
	Name   string
	Email  string
	Status string
}

// FromEntity 从领域实体构建 UserResult。
func FromEntity(e *domainIdentity.User) *UserResult {
	return &UserResult{
		ID:     e.GetID(),
		Name:   e.GetName(),
		Email:  e.GetEmail(),
		Status: statusName(e.GetStatus()),
	}
}

func statusName(status domainIdentity.Status) string {
	switch status {
	case domainIdentity.StatusPendingVerification:
		return "pending_verification"
	case domainIdentity.StatusActive:
		return "active"
	case domainIdentity.StatusLocked:
		return "locked"
	case domainIdentity.StatusBanned:
		return "banned"
	case domainIdentity.StatusDeleted:
		return "deleted"
	default:
		return "unknown"
	}
}
