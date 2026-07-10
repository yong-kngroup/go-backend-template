package identity

import (
	"context"
	"errors"
	"strconv"
	"time"

	domainAudit "github.com/freeDog-wy/go-backend-template/internal/domain/audit"
	domainAuth "github.com/freeDog-wy/go-backend-template/internal/domain/auth"
	domainAuthorization "github.com/freeDog-wy/go-backend-template/internal/domain/authorization"
	domainIdentity "github.com/freeDog-wy/go-backend-template/internal/domain/identity"
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	domainVerification "github.com/freeDog-wy/go-backend-template/internal/domain/verification"
	"github.com/freeDog-wy/go-backend-template/pkg/captcha"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

var identityTracer = otel.Tracer("github.com/freeDog-wy/go-backend-template/internal/usecase/identity")

// EmailVerificationIssuer 为 identity 注册流程提供邮箱验证签发能力。
type EmailVerificationIssuer interface {
	IssueEmailVerification(ctx context.Context, user *domainIdentity.User) (domainVerification.EmailVerificationRequested, error)
}

type Service struct {
	tx                 shared.TxManager
	userRepo           domainIdentity.Repository
	authorizationRepo  domainAuthorization.Repository
	roleBindingSvc     *domainAuthorization.RoleBindingService
	credentialRepo     domainAuth.CredentialRepository
	pwdHasher          shared.PasswordHasher
	captcha            captcha.Generator
	verificationIssuer EmailVerificationIssuer
	logger             logger.Logger
	eventBus           shared.EventBus
}

func New(
	tx shared.TxManager,
	userRepo domainIdentity.Repository,
	authorizationRepo domainAuthorization.Repository,
	credentialRepo domainAuth.CredentialRepository,
	pwdHasher shared.PasswordHasher,
	captcha captcha.Generator,
	verificationIssuer EmailVerificationIssuer,
	logger logger.Logger,
	eventBus shared.EventBus,
) *Service {
	return &Service{
		tx:                 tx,
		userRepo:           userRepo,
		authorizationRepo:  authorizationRepo,
		roleBindingSvc:     domainAuthorization.NewRoleBindingService(),
		credentialRepo:     credentialRepo,
		pwdHasher:          pwdHasher,
		captcha:            captcha,
		verificationIssuer: verificationIssuer,
		logger:             logger,
		eventBus:           eventBus,
	}
}

// Register 用户注册——编排验证码校验、邮箱唯一检查、密码哈希、
// 实体创建、事务持久化、领域事件发布。
func (s *Service) Register(ctx context.Context, cmd RegisterCmd) (result *UserResult, err error) {
	ctx, span := identityTracer.Start(ctx, "identity.register")
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
			if result != nil {
				span.SetAttributes(attribute.Int64("app.user.id", int64(result.ID)))
			}
		}
		span.End()
	}()

	if !s.captcha.Verify(cmd.CaptchaID, cmd.CaptchaCode) {
		return nil, ErrInvalidCaptcha
	}

	if _, err := s.userRepo.FindByEmail(ctx, cmd.Email); err == nil {
		return nil, ErrEmailTaken
	} else if !errors.Is(err, shared.ErrNotFound) {
		return nil, err
	}

	hashed, err := s.pwdHasher.Hash(cmd.Password)
	if err != nil {
		return nil, err
	}

	user, err := domainIdentity.NewUser(cmd.Name, cmd.Email)
	if err != nil {
		return nil, err
	}

	err = s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.userRepo.Create(ctx, user); err != nil {
			return err
		}

		credential, err := domainAuth.NewUserCredential(user.GetID(), hashed, time.Now())
		if err != nil {
			return err
		}
		if err := s.credentialRepo.Create(ctx, credential); err != nil {
			return err
		}

		verificationEvent, err := s.verificationIssuer.IssueEmailVerification(ctx, user)
		if err != nil {
			return err
		}

		events := append([]shared.Event{}, user.Events()...)
		events = append(events, verificationEvent)
		if err := s.eventBus.Publish(ctx, events...); err != nil {
			return err
		}
		user.ClearEvents()
		result = FromEntity(user)
		return nil
	})
	if err != nil {
		return nil, err
	}

	if s.logger != nil {
		s.logger.Info("user registered", "user_id", result.ID, "email", cmd.Email)
	}
	return result, nil
}

func (s *Service) CreateAdminUser(ctx context.Context, cmd CreateAdminUserCmd) (*UserResult, error) {
	var result *UserResult
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		var err error
		result, err = s.createAdminUserInTx(ctx, cmd)
		return err
	})
	if err != nil {
		return nil, err
	}

	if s.logger != nil {
		s.logger.Info("admin user created", "user_id", result.ID, "email", cmd.Email)
	}
	return result, nil
}

func (s *Service) createAdminUserInTx(ctx context.Context, cmd CreateAdminUserCmd) (*UserResult, error) {
	if _, err := s.userRepo.FindByEmail(ctx, cmd.Email); err == nil {
		return nil, ErrEmailTaken
	} else if !errors.Is(err, shared.ErrNotFound) {
		return nil, err
	}

	hashed, err := s.pwdHasher.Hash(cmd.Password)
	if err != nil {
		return nil, err
	}

	user, err := domainIdentity.NewAdminUser(cmd.Name, cmd.Email)
	if err != nil {
		return nil, err
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	credential, err := domainAuth.NewUserCredential(user.GetID(), hashed, time.Now())
	if err != nil {
		return nil, err
	}
	if err := s.credentialRepo.Create(ctx, credential); err != nil {
		return nil, err
	}

	if err := s.bindRoles(ctx, user.GetID(), cmd.RoleIDs); err != nil {
		return nil, err
	}

	return FromEntity(user), nil
}

func (s *Service) bindRoles(ctx context.Context, userID uint, roleIDs []uint) error {
	if len(roleIDs) == 0 {
		return nil
	}

	roles, err := s.authorizationRepo.FindRolesByIDs(ctx, roleIDs)
	if err != nil {
		return err
	}
	preparedRoleIDs, err := s.roleBindingSvc.PrepareRoleIDs(roleIDs, roles)
	if err != nil {
		return err
	}
	return s.authorizationRepo.ReplaceUserRoles(ctx, userID, preparedRoleIDs)
}

func (s *Service) GetByID(ctx context.Context, userID uint) (*UserResult, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return FromEntity(user), nil
}

func (s *Service) List(ctx context.Context, cmd ListUsersCmd) ([]*UserResult, shared.PageResult, error) {
	users, total, err := s.userRepo.List(ctx, cmd.Page)
	if err != nil {
		return nil, shared.PageResult{}, err
	}
	results := make([]*UserResult, 0, len(users))
	for _, user := range users {
		results = append(results, FromEntity(user))
	}
	return results, shared.PageResult{
		Page:    cmd.Page.Page,
		PerPage: cmd.Page.PerPage,
		Total:   total,
	}, nil
}

func (s *Service) UpdateProfile(ctx context.Context, cmd UpdateProfileCmd) (*UserResult, error) {
	user, err := s.userRepo.FindByID(ctx, cmd.UserID)
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	if err := user.UpdateProfile(cmd.Name); err != nil {
		return nil, err
	}
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}
	return FromEntity(user), nil
}

func (s *Service) UpdateStatus(ctx context.Context, cmd UpdateStatusCmd) (*UserResult, error) {
	user, err := s.userRepo.FindByID(ctx, cmd.UserID)
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	oldStatus := statusName(user.GetStatus())

	switch cmd.Status {
	case "active":
		if err := user.Activate(); err != nil {
			return nil, err
		}
	case "locked":
		if err := user.Lock(); err != nil {
			return nil, err
		}
	case "banned":
		user.Ban()
	default:
		return nil, domainIdentity.ErrInvalidUserData
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}
	s.publishAudit(ctx, domainAudit.LogRequested{
		ActorUserID: uintPtr(cmd.ActorUserID),
		TargetType:  "user",
		TargetID:    uintString(user.GetID()),
		Action:      domainAudit.ActionUserStatusChanged,
		Result:      domainAudit.ResultSuccess,
		IP:          cmd.IP,
		UserAgent:   cmd.UserAgent,
		Metadata: map[string]any{
			"old_status": oldStatus,
			"new_status": cmd.Status,
		},
	})
	return FromEntity(user), nil
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
