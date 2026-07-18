package main

import (
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	platformAudit "github.com/freeDog-wy/go-backend-template/internal/platform/audit"
	platformIdempotency "github.com/freeDog-wy/go-backend-template/internal/platform/idempotency"
	platformOutbox "github.com/freeDog-wy/go-backend-template/internal/platform/outbox"
	repoAuth "github.com/freeDog-wy/go-backend-template/internal/repository/auth"
	repoAuthorization "github.com/freeDog-wy/go-backend-template/internal/repository/authorization"
	repoCMS "github.com/freeDog-wy/go-backend-template/internal/repository/cms"
	repoIdentity "github.com/freeDog-wy/go-backend-template/internal/repository/identity"
	repoMedia "github.com/freeDog-wy/go-backend-template/internal/repository/media"
	repoServiceAccount "github.com/freeDog-wy/go-backend-template/internal/repository/service_account"
	repoVerification "github.com/freeDog-wy/go-backend-template/internal/repository/verification"
	svcAuth "github.com/freeDog-wy/go-backend-template/internal/usecase/auth"
	svcAuthorization "github.com/freeDog-wy/go-backend-template/internal/usecase/authorization"
	svcBootstrap "github.com/freeDog-wy/go-backend-template/internal/usecase/bootstrap"
	svcCMS "github.com/freeDog-wy/go-backend-template/internal/usecase/cms"
	svcIdentity "github.com/freeDog-wy/go-backend-template/internal/usecase/identity"
	svcMedia "github.com/freeDog-wy/go-backend-template/internal/usecase/media"
	svcVerification "github.com/freeDog-wy/go-backend-template/internal/usecase/verification"
	"gorm.io/gorm"
)

type serverRepositories struct {
	credential        *repoAuth.CredentialRepository
	authorization     *repoAuthorization.Repository
	user              *repoIdentity.Repository
	verification      *repoVerification.Repository
	cms               *repoCMS.Repository
	media             *repoMedia.Repository
	mcpServiceAccount *repoServiceAccount.ServiceAccountRepository
}

func newServerRepositories(db *gorm.DB) *serverRepositories {
	return &serverRepositories{
		credential:        repoAuth.New(db),
		authorization:     repoAuthorization.New(db),
		user:              repoIdentity.New(db),
		verification:      repoVerification.New(db),
		cms:               repoCMS.New(db),
		media:             repoMedia.New(db),
		mcpServiceAccount: repoServiceAccount.New(db),
	}
}

type serverPlatform struct {
	outbox      *platformOutbox.Repository
	idempotency *platformIdempotency.Repository
	audit       *platformAudit.Repository
}

func newServerPlatform(db *gorm.DB) *serverPlatform {
	return &serverPlatform{
		outbox:      platformOutbox.New(db),
		idempotency: platformIdempotency.New(db),
		audit:       platformAudit.New(db),
	}
}

type serverServices struct {
	verification  *svcVerification.Service
	authorization *svcAuthorization.Service
	bootstrap     *svcBootstrap.Service
	identity      *svcIdentity.Service
	auth          *svcAuth.Service
	cms           *svcCMS.Service
	media         *svcMedia.Service
}

func newServerServices(cfg *config.Config, infra *serverInfrastructure, repos *serverRepositories, platform *serverPlatform) (*serverServices, error) {
	eventBus := newServerEventBus(platform.outbox)
	auditRecorder := platformAudit.NewRecorder(platform.audit)
	verification := svcVerification.New(infra.txManager, repos.user, repos.verification, repos.credential, infra.passwordHasher, infra.sessionStore, eventBus, infra.logger, auditRecorder)
	authorization := svcAuthorization.New(infra.txManager, repos.authorization, repos.user, eventBus, infra.logger, auditRecorder)
	bootstrap := svcBootstrap.New(infra.txManager, repos.user, repos.authorization, repos.credential, infra.passwordHasher, infra.logger)
	identity := svcIdentity.New(infra.txManager, repos.user, repos.authorization, repos.credential, infra.passwordHasher, infra.captcha, verification, infra.logger, eventBus, auditRecorder)
	auth := svcAuth.New(
		repos.user,
		repos.credential,
		infra.sessionStore,
		infra.passwordHasher,
		infra.tokenManager,
		eventBus,
		infra.logger,
		cfg.Auth.JWTIssuer,
		cfg.Auth.JWTAudience,
		time.Duration(cfg.Auth.AccessTokenTTLMinutes)*time.Minute,
		time.Duration(cfg.Auth.RefreshTokenTTLHours)*time.Hour,
		auditRecorder,
	)
	auth.SetTxManager(infra.txManager)
	cms := svcCMS.New(infra.txManager, repos.cms)
	cms.SetAuditRecorder(auditRecorder)
	media := svcMedia.New(infra.txManager, repos.media, infra.mediaStorage)
	cms.SetMediaFinder(media)
	cms.SetPublicMediaFinder(media)

	return &serverServices{
		verification:  verification,
		authorization: authorization,
		bootstrap:     bootstrap,
		identity:      identity,
		auth:          auth,
		cms:           cms,
		media:         media,
	}, nil
}
