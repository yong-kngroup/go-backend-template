package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	domainAuth "github.com/freeDog-wy/go-backend-template/internal/domain/auth"
	domainAuthorization "github.com/freeDog-wy/go-backend-template/internal/domain/authorization"
	domainIdentity "github.com/freeDog-wy/go-backend-template/internal/domain/identity"
	domainMCP "github.com/freeDog-wy/go-backend-template/internal/domain/mcp"
	domainShared "github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
)

const serviceRoleCode = "cms_mcp_operator"

var servicePermissionCodes = []string{
	"cms.article.create",
	"cms.article.update",
	"cms.article.publish",
	"cms.category.manage",
	"cms.locale.manage",
	"cms.tag.manage",
}

type BootstrapService struct {
	tx         domainShared.TxManager
	accounts   domainMCP.ServiceAccountRepository
	users      domainIdentity.Repository
	authorizer domainAuthorization.Repository
	hasher     domainShared.PasswordHasher
	sessions   domainAuth.SessionStore
	logger     logger.Logger
}

type BootstrapCmd struct {
	Enabled               bool
	Name                  string
	Email                 string
	ClientID              string
	ClientSecret          string
	RotationGrace         time.Duration
	ServiceAccountEnabled bool
}

func NewBootstrapService(tx domainShared.TxManager, accounts domainMCP.ServiceAccountRepository, users domainIdentity.Repository, authorizer domainAuthorization.Repository, hasher domainShared.PasswordHasher, sessions domainAuth.SessionStore, log logger.Logger) *BootstrapService {
	return &BootstrapService{tx: tx, accounts: accounts, users: users, authorizer: authorizer, hasher: hasher, sessions: sessions, logger: log}
}

func (s *BootstrapService) Bootstrap(ctx context.Context, cmd BootstrapCmd) error {
	if !cmd.Enabled {
		return nil
	}
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Email = strings.ToLower(strings.TrimSpace(cmd.Email))
	cmd.ClientID = strings.TrimSpace(cmd.ClientID)
	if cmd.Name == "" || cmd.Email == "" || cmd.ClientID == "" || len(cmd.ClientSecret) < 32 || cmd.RotationGrace <= 0 {
		return fmt.Errorf("mcp service account name, email, client ID, and a 32-byte client secret are required when enabled")
	}

	var userID uint
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		user, err := s.users.FindByEmail(ctx, cmd.Email)
		if errors.Is(err, domainShared.ErrNotFound) {
			user, err = domainIdentity.NewAdminUser(cmd.Name, cmd.Email)
			if err != nil {
				return err
			}
			if err := s.users.Create(ctx, user); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		userID = user.GetID()

		account, err := s.accounts.FindByClientID(ctx, cmd.ClientID)
		if errors.Is(err, domainShared.ErrNotFound) {
			if existing, findErr := s.accounts.FindByUserID(ctx, user.GetID()); findErr == nil {
				return fmt.Errorf("mcp service account user is already bound to client ID %q", existing.GetClientID())
			} else if !errors.Is(findErr, domainShared.ErrNotFound) {
				return findErr
			}
			hash, hashErr := s.hasher.Hash(cmd.ClientSecret)
			if hashErr != nil {
				return hashErr
			}
			account, err = domainMCP.NewServiceAccount(user.GetID(), cmd.ClientID, hash, time.Now())
			if err != nil {
				return err
			}
			if err := s.accounts.Create(ctx, account); err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if account.GetUserID() != user.GetID() {
			return fmt.Errorf("mcp service account configuration does not match the existing account")
		} else if !s.matchesConfiguredSecret(account, cmd.ClientSecret) {
			hash, hashErr := s.hasher.Hash(cmd.ClientSecret)
			if hashErr != nil {
				return hashErr
			}
			if err := account.RotateSecret(hash, time.Now().Add(cmd.RotationGrace), time.Now()); err != nil {
				return err
			}
			if err := s.accounts.Update(ctx, account); err != nil {
				return err
			}
			if s.sessions != nil {
				if err := s.sessions.DeleteByUserID(ctx, user.GetID()); err != nil {
					return err
				}
			}
		}

		if cmd.ServiceAccountEnabled && !account.IsEnabled() {
			if err := account.Enable(time.Now()); err != nil {
				return err
			}
			if err := s.accounts.Update(ctx, account); err != nil {
				return err
			}
		}
		if !cmd.ServiceAccountEnabled && account.IsEnabled() {
			if err := account.Disable(time.Now()); err != nil {
				return err
			}
			if err := s.accounts.Update(ctx, account); err != nil {
				return err
			}
			if s.sessions != nil {
				if err := s.sessions.DeleteByUserID(ctx, user.GetID()); err != nil {
					return err
				}
			}
		}

		role, err := domainAuthorization.NewRole(serviceRoleCode, "CMS MCP Operator", "Private local MCP service account")
		if err != nil {
			return err
		}
		role, err = s.authorizer.EnsureRole(ctx, role)
		if err != nil {
			return err
		}
		permissions, err := s.authorizer.FindPermissionsByCodes(ctx, servicePermissionCodes)
		if err != nil {
			return err
		}
		if len(permissions) != len(servicePermissionCodes) {
			return fmt.Errorf("mcp service role permissions are not initialized")
		}
		permissionIDs := make([]uint, 0, len(permissions))
		for _, permission := range permissions {
			permissionIDs = append(permissionIDs, permission.GetID())
		}
		if err := s.authorizer.ReplaceRolePermissions(ctx, role.GetID(), permissionIDs); err != nil {
			return err
		}
		return s.authorizer.ReplaceUserRoles(ctx, user.GetID(), []uint{role.GetID()})
	})
	if err != nil {
		return err
	}
	if s.logger != nil {
		s.logger.Info("mcp service account ready", "user_id", userID, "client_id", cmd.ClientID)
	}
	return nil
}

func (s *BootstrapService) matchesConfiguredSecret(account *domainMCP.ServiceAccount, secret string) bool {
	return s.hasher.Verify(secret, account.GetClientSecretHash()) || (account.PreviousSecretActive(time.Now()) && s.hasher.Verify(secret, account.GetPreviousClientSecretHash()))
}
