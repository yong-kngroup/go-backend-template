package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	domainAuth "github.com/freeDog-wy/go-backend-template/internal/domain/auth"
	domainAuthorization "github.com/freeDog-wy/go-backend-template/internal/domain/authorization"
	domainIdentity "github.com/freeDog-wy/go-backend-template/internal/domain/identity"
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
)

type Service struct {
	tx                shared.TxManager
	userRepo          domainIdentity.Repository
	authorizationRepo domainAuthorization.Repository
	roleBindingSvc    *domainAuthorization.RoleBindingService
	credentialRepo    domainAuth.CredentialRepository
	passwordHasher    shared.PasswordHasher
	logger            logger.Logger
}

func New(
	tx shared.TxManager,
	userRepo domainIdentity.Repository,
	authorizationRepo domainAuthorization.Repository,
	credentialRepo domainAuth.CredentialRepository,
	passwordHasher shared.PasswordHasher,
	logger logger.Logger,
) *Service {
	return &Service{
		tx:                tx,
		userRepo:          userRepo,
		authorizationRepo: authorizationRepo,
		roleBindingSvc:    domainAuthorization.NewRoleBindingService(),
		credentialRepo:    credentialRepo,
		passwordHasher:    passwordHasher,
		logger:            logger,
	}
}

// InitializeAuthorization 幂等地安装系统默认授权数据。
// 服务进程必须在开始处理请求前调用它；授权管理用例不执行懒初始化，以保持运行时职责
// 与启动初始化职责分离。
func (s *Service) InitializeAuthorization(ctx context.Context) error {
	return s.tx.Do(ctx, func(ctx context.Context) error {
		return initializeAuthorizationDefaults(ctx, s.authorizationRepo)
	})
}

// BootstrapAdmin 按配置创建首个管理员。调用方必须先执行 InitializeAuthorization，
// 以确保超级管理员角色已存在。
func (s *Service) BootstrapAdmin(ctx context.Context, cmd BootstrapAdminCmd) error {
	var hasSuperAdmin bool
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		count, err := s.authorizationRepo.CountUsersByRoleCode(ctx, domainAuthorization.SuperAdminRoleCode)
		if err != nil {
			return err
		}
		if count > 0 {
			hasSuperAdmin = true
			return nil
		}

		if !cmd.Enabled {
			return nil
		}

		if err := s.createBootstrapAdmin(ctx, cmd); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	if hasSuperAdmin {
		return nil
	}

	if !cmd.Enabled && s.logger != nil {
		s.logger.Info("bootstrap admin skipped", "reason", "disabled")
	}

	return nil
}

func (s *Service) createBootstrapAdmin(ctx context.Context, cmd BootstrapAdminCmd) error {
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		name = "Admin"
	}
	email := strings.TrimSpace(cmd.Email)
	password := strings.TrimSpace(cmd.Password)
	if email == "" || password == "" {
		return fmt.Errorf("bootstrap admin email and password are required when enabled")
	}

	if _, err := s.userRepo.FindByEmail(ctx, email); err == nil {
		return fmt.Errorf("bootstrap admin email already exists: %s", email)
	} else if !errors.Is(err, shared.ErrNotFound) {
		return err
	}

	hashedPassword, err := s.passwordHasher.Hash(password)
	if err != nil {
		return err
	}

	user, err := domainIdentity.NewAdminUser(name, email)
	if err != nil {
		return err
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		return err
	}

	credential, err := domainAuth.NewUserCredential(user.GetID(), hashedPassword, time.Now())
	if err != nil {
		return err
	}
	if err := s.credentialRepo.Create(ctx, credential); err != nil {
		return err
	}

	role, err := s.authorizationRepo.FindRoleByCode(ctx, domainAuthorization.SuperAdminRoleCode)
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			return domainAuthorization.ErrRoleNotFound
		}
		return err
	}

	roleIDs, err := s.roleBindingSvc.PrepareRoleIDs([]uint{role.GetID()}, []*domainAuthorization.Role{role})
	if err != nil {
		return err
	}
	if err := s.authorizationRepo.ReplaceUserRoles(ctx, user.GetID(), roleIDs); err != nil {
		return err
	}

	if s.logger != nil {
		s.logger.Info("bootstrap admin created", "email", email, "user_id", user.GetID())
	}
	return nil
}
