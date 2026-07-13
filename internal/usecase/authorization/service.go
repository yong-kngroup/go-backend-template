package authorization

import (
	"context"
	"errors"
	"slices"
	"strconv"

	domainAudit "github.com/freeDog-wy/go-backend-template/internal/domain/audit"
	domainAuthorization "github.com/freeDog-wy/go-backend-template/internal/domain/authorization"
	domainIdentity "github.com/freeDog-wy/go-backend-template/internal/domain/identity"
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	usecaseSupport "github.com/freeDog-wy/go-backend-template/internal/usecase/support"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
)

type Service struct {
	tx       shared.TxManager
	repo     domainAuthorization.Repository
	userRepo domainIdentity.Repository
	roleBind domainAuthorization.RoleBindingService
	defaults *usecaseSupport.AuthorizationDefaultsInstaller
	eventBus shared.EventBus
	logger   logger.Logger
}

func New(
	tx shared.TxManager,
	repo domainAuthorization.Repository,
	userRepo domainIdentity.Repository,
	eventBus shared.EventBus,
	logger logger.Logger,
) *Service {
	return &Service{
		tx:       tx,
		repo:     repo,
		userRepo: userRepo,
		roleBind: *domainAuthorization.NewRoleBindingService(),
		defaults: usecaseSupport.NewAuthorizationDefaultsInstaller(repo),
		eventBus: eventBus,
		logger:   logger,
	}
}

func (s *Service) EnsureAdminAccess(ctx context.Context, userID uint) error {
	codes, err := s.repo.ListUserPermissionCodes(ctx, userID)
	if err != nil {
		return err
	}
	if len(codes) == 0 {
		return domainAuthorization.ErrPermissionDenied
	}
	return nil
}

func (s *Service) HasPermission(ctx context.Context, userID uint, code string) (bool, error) {
	codes, err := s.repo.ListUserPermissionCodes(ctx, userID)
	if err != nil {
		return false, err
	}
	return slices.Contains(codes, code), nil
}

func (s *Service) ListRoles(ctx context.Context, cmd ListRolesCmd) ([]*RoleResult, shared.PageResult, error) {
	roles, total, err := s.repo.ListRoles(ctx, cmd.Page)
	if err != nil {
		return nil, shared.PageResult{}, err
	}

	results := make([]*RoleResult, 0, len(roles))
	for _, role := range roles {
		permissions, err := s.repo.ListRolePermissions(ctx, role.GetID())
		if err != nil {
			return nil, shared.PageResult{}, err
		}
		results = append(results, toRoleResult(role, permissions))
	}
	return results, shared.PageResult{
		Page:    cmd.Page.Page,
		PerPage: cmd.Page.PerPage,
		Total:   total,
	}, nil
}

func (s *Service) CreateRole(ctx context.Context, cmd CreateRoleCmd) (*RoleResult, error) {
	role, err := domainAuthorization.NewRole(cmd.Code, cmd.Name, cmd.Description)
	if err != nil {
		return nil, err
	}

	var result *RoleResult
	err = s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.EnsureDefaults(ctx); err != nil {
			return err
		}
		if err := s.repo.CreateRole(ctx, role); err != nil {
			return err
		}
		permissions, err := s.repo.FindPermissionsByCodes(ctx, cmd.PermissionCodes)
		if err != nil {
			return err
		}
		permissionIDs := make([]uint, 0, len(permissions))
		for _, permission := range permissions {
			permissionIDs = append(permissionIDs, permission.GetID())
		}
		if err := s.repo.ReplaceRolePermissions(ctx, role.GetID(), permissionIDs); err != nil {
			return err
		}
		result = toRoleResult(role, permissions)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) UpdateRole(ctx context.Context, cmd UpdateRoleCmd) (*RoleResult, error) {
	var result *RoleResult
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.EnsureDefaults(ctx); err != nil {
			return err
		}
		role, err := s.repo.FindRoleByID(ctx, cmd.RoleID)
		if err != nil {
			if errors.Is(err, shared.ErrNotFound) {
				return domainAuthorization.ErrRoleNotFound
			}
			return err
		}

		if err := role.Update(cmd.Name, cmd.Description); err != nil {
			return err
		}
		if err := s.repo.UpdateRole(ctx, role); err != nil {
			return err
		}

		permissions, err := s.repo.FindPermissionsByCodes(ctx, cmd.PermissionCodes)
		if err != nil {
			return err
		}
		permissionIDs := make([]uint, 0, len(permissions))
		for _, permission := range permissions {
			permissionIDs = append(permissionIDs, permission.GetID())
		}
		if err := s.repo.ReplaceRolePermissions(ctx, role.GetID(), permissionIDs); err != nil {
			return err
		}
		result = toRoleResult(role, permissions)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) ListPermissions(ctx context.Context, cmd ListPermissionsCmd) ([]*PermissionResult, shared.PageResult, error) {
	permissions, total, err := s.repo.ListPermissions(ctx, cmd.Page)
	if err != nil {
		return nil, shared.PageResult{}, err
	}
	results := make([]*PermissionResult, 0, len(permissions))
	for _, permission := range permissions {
		results = append(results, &PermissionResult{
			ID:          permission.GetID(),
			Code:        permission.GetCode(),
			Name:        permission.GetName(),
			Description: permission.GetDescription(),
		})
	}
	return results, shared.PageResult{
		Page:    cmd.Page.Page,
		PerPage: cmd.Page.PerPage,
		Total:   total,
	}, nil
}

func (s *Service) ReplaceUserRoles(ctx context.Context, cmd ReplaceUserRolesCmd) error {
	if err := s.EnsureDefaults(ctx); err != nil {
		return err
	}
	if _, err := s.userRepo.FindByID(ctx, cmd.UserID); err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			return domainIdentity.ErrUserNotFound
		}
		return err
	}
	roleIDs, err := s.validateRoleIDs(ctx, cmd.RoleIDs)
	if err != nil {
		return err
	}
	if err := s.repo.ReplaceUserRoles(ctx, cmd.UserID, roleIDs); err != nil {
		return err
	}
	s.publishAudit(ctx, domainAudit.LogRequested{
		ActorUserID: uintPtr(cmd.ActorUserID),
		TargetType:  "user",
		TargetID:    uintString(cmd.UserID),
		Action:      domainAudit.ActionUserRolesChanged,
		Result:      domainAudit.ResultSuccess,
		IP:          cmd.IP,
		UserAgent:   cmd.UserAgent,
		Metadata: map[string]any{
			"role_ids": roleIDs,
		},
	})
	return nil
}

func (s *Service) ListUserRoles(ctx context.Context, userID uint) ([]*RoleResult, error) {
	roles, err := s.repo.ListUserRoles(ctx, userID)
	if err != nil {
		return nil, err
	}
	results := make([]*RoleResult, 0, len(roles))
	for _, role := range roles {
		permissions, err := s.repo.ListRolePermissions(ctx, role.GetID())
		if err != nil {
			return nil, err
		}
		results = append(results, toRoleResult(role, permissions))
	}
	return results, nil
}

func (s *Service) EnsureDefaults(ctx context.Context) error {
	return s.defaults.Ensure(ctx)
}

func (s *Service) validateRoleIDs(ctx context.Context, roleIDs []uint) ([]uint, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}

	roles, err := s.repo.FindRolesByIDs(ctx, roleIDs)
	if err != nil {
		return nil, err
	}
	return s.roleBind.PrepareRoleIDs(roleIDs, roles)
}

func (s *Service) publishAudit(ctx context.Context, evt domainAudit.LogRequested) {
	if s.eventBus == nil {
		return
	}
	if err := s.eventBus.Publish(ctx, evt); err != nil && s.logger != nil {
		s.logger.Error("publish audit event failed", "action", evt.Action, "error", err)
	}
}

func uintPtr(value uint) *uint {
	if value == 0 {
		return nil
	}
	return &value
}

func uintString(value uint) string {
	return strconv.FormatUint(uint64(value), 10)
}

func toRoleResult(role *domainAuthorization.Role, permissions []*domainAuthorization.Permission) *RoleResult {
	codes := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		codes = append(codes, permission.GetCode())
	}
	slices.Sort(codes)
	return &RoleResult{
		ID:              role.GetID(),
		Code:            role.GetCode(),
		Name:            role.GetName(),
		Description:     role.GetDescription(),
		PermissionCodes: codes,
	}
}
