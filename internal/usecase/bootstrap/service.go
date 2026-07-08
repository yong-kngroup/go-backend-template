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
	usecaseSupport "github.com/freeDog-wy/go-backend-template/internal/usecase/support"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
)

type Service struct {
	tx                shared.TxManager
	userRepo          domainIdentity.Repository
	authorizationRepo domainAuthorization.Repository
	roleBindingSvc    *domainAuthorization.RoleBindingService
	defaults          *usecaseSupport.AuthorizationDefaultsInstaller
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
		defaults:          usecaseSupport.NewAuthorizationDefaultsInstaller(authorizationRepo),
		credentialRepo:    credentialRepo,
		passwordHasher:    passwordHasher,
		logger:            logger,
	}
}

func (s *Service) BootstrapAdmin(ctx context.Context, cmd BootstrapAdminCmd) error {
	var hasSuperAdmin bool
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.defaults.Ensure(ctx); err != nil {
			return err
		}

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
