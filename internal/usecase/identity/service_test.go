package identity

import (
	"context"
	"errors"
	"testing"

	domainAuth "github.com/freeDog-wy/go-backend-template/internal/domain/auth"
	domainAuthorization "github.com/freeDog-wy/go-backend-template/internal/domain/authorization"
	domainIdentity "github.com/freeDog-wy/go-backend-template/internal/domain/identity"
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	domainVerification "github.com/freeDog-wy/go-backend-template/internal/domain/verification"
)

func TestRegister(t *testing.T) {
	t.Parallel()

	t.Run("persists user credential and outbox events in a transaction", func(t *testing.T) {
		t.Parallel()

		tx := &identityTx{}
		users := &identityUserRepo{findByEmailErr: shared.ErrNotFound, assignID: 42}
		credentials := &identityCredentialRepo{}
		bus := &identityEventBus{}
		service := New(tx, users, nil, credentials, &identityHasher{}, &identityCaptcha{valid: true}, identityIssuer{}, nil, bus)

		result, err := service.Register(context.Background(), RegisterCmd{
			Name: " Alice ", Email: "ALICE@example.com", Password: "password", CaptchaID: "captcha", CaptchaCode: "123456",
		})
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		if !tx.called || result.ID != 42 || result.Email != "alice@example.com" || result.Status != "pending_verification" {
			t.Fatalf("unexpected result: tx=%v result=%+v", tx.called, result)
		}
		if users.created == nil || credentials.created == nil || credentials.created.GetUserID() != 42 {
			t.Fatal("user and credential were not persisted with the assigned user ID")
		}
		if len(bus.events) != 2 {
			t.Fatalf("published events = %d, want 2", len(bus.events))
		}
		registered, ok := bus.events[0].(domainIdentity.Registered)
		if !ok || registered.UserID != 42 || registered.Email != "alice@example.com" {
			t.Fatalf("registered event = %#v", bus.events[0])
		}
		verification, ok := bus.events[1].(domainVerification.EmailVerificationRequested)
		if !ok || verification.UserID != 42 {
			t.Fatalf("verification event = %#v", bus.events[1])
		}
	})

	t.Run("rejects invalid captcha before accessing dependencies", func(t *testing.T) {
		t.Parallel()
		users := &identityUserRepo{}
		service := New(&identityTx{}, users, nil, nil, nil, &identityCaptcha{}, nil, nil, nil)

		_, err := service.Register(context.Background(), RegisterCmd{CaptchaID: "id", CaptchaCode: "bad"})
		if !errors.Is(err, ErrInvalidCaptcha) || users.findByEmailCalls != 0 {
			t.Fatalf("Register() error = %v, find calls = %d", err, users.findByEmailCalls)
		}
	})

	t.Run("returns email taken without creating records", func(t *testing.T) {
		t.Parallel()
		user, _ := domainIdentity.NewUser("Alice", "alice@example.com")
		users := &identityUserRepo{found: user}
		service := New(&identityTx{}, users, nil, nil, nil, &identityCaptcha{valid: true}, nil, nil, nil)

		_, err := service.Register(context.Background(), RegisterCmd{Name: "Alice", Email: "alice@example.com"})
		if !errors.Is(err, ErrEmailTaken) || users.created != nil {
			t.Fatalf("Register() error = %v, created = %#v", err, users.created)
		}
	})
}

type identityTx struct{ called bool }

func (t *identityTx) Do(ctx context.Context, fn func(context.Context) error) error {
	t.called = true
	return fn(ctx)
}

type identityUserRepo struct {
	found            *domainIdentity.User
	findByEmailErr   error
	created          *domainIdentity.User
	assignID         uint
	findByEmailCalls int
}

func (r *identityUserRepo) FindByID(context.Context, uint) (*domainIdentity.User, error) {
	return nil, shared.ErrNotFound
}
func (r *identityUserRepo) FindByEmail(context.Context, string) (*domainIdentity.User, error) {
	r.findByEmailCalls++
	if r.findByEmailErr != nil {
		return nil, r.findByEmailErr
	}
	return r.found, nil
}
func (r *identityUserRepo) List(context.Context, shared.PageQuery) ([]*domainIdentity.User, int64, error) {
	return nil, 0, nil
}
func (r *identityUserRepo) Create(_ context.Context, user *domainIdentity.User) error {
	r.created = user
	user.AssignID(r.assignID)
	return nil
}
func (r *identityUserRepo) Update(context.Context, *domainIdentity.User) error { return nil }
func (r *identityUserRepo) Delete(context.Context, uint) error                 { return nil }

type identityCredentialRepo struct{ created *domainAuth.UserCredential }

func (r *identityCredentialRepo) Create(_ context.Context, credential *domainAuth.UserCredential) error {
	r.created = credential
	return nil
}
func (r *identityCredentialRepo) FindByUserID(context.Context, uint) (*domainAuth.UserCredential, error) {
	return nil, shared.ErrNotFound
}
func (r *identityCredentialRepo) Update(context.Context, *domainAuth.UserCredential) error {
	return nil
}

type identityHasher struct{}

func (*identityHasher) Hash(string) (string, error) { return "hash", nil }
func (*identityHasher) Verify(string, string) bool  { return true }

type identityCaptcha struct{ valid bool }

func (*identityCaptcha) Generate() (string, string, error) { return "", "", nil }
func (c *identityCaptcha) Verify(string, string) bool      { return c.valid }

type identityIssuer struct{}

func (identityIssuer) IssueEmailVerification(_ context.Context, user *domainIdentity.User) (domainVerification.EmailVerificationRequested, error) {
	return domainVerification.EmailVerificationRequested{UserID: user.GetID(), Email: user.GetEmail(), Token: "token"}, nil
}

type identityEventBus struct{ events []shared.Event }

func (b *identityEventBus) Publish(_ context.Context, events ...shared.Event) error {
	b.events = append(b.events, events...)
	return nil
}

var _ domainAuthorization.Repository = (*identityAuthorizationRepo)(nil)

type identityAuthorizationRepo struct{}

func (*identityAuthorizationRepo) EnsurePermissions(context.Context, []*domainAuthorization.Permission) error {
	return nil
}
func (*identityAuthorizationRepo) EnsureRole(context.Context, *domainAuthorization.Role) (*domainAuthorization.Role, error) {
	return nil, nil
}
func (*identityAuthorizationRepo) ListRoles(context.Context, shared.PageQuery) ([]*domainAuthorization.Role, int64, error) {
	return nil, 0, nil
}
func (*identityAuthorizationRepo) FindRoleByID(context.Context, uint) (*domainAuthorization.Role, error) {
	return nil, nil
}
func (*identityAuthorizationRepo) FindRoleByCode(context.Context, string) (*domainAuthorization.Role, error) {
	return nil, nil
}
func (*identityAuthorizationRepo) FindRolesByIDs(context.Context, []uint) ([]*domainAuthorization.Role, error) {
	return nil, nil
}
func (*identityAuthorizationRepo) CreateRole(context.Context, *domainAuthorization.Role) error {
	return nil
}
func (*identityAuthorizationRepo) UpdateRole(context.Context, *domainAuthorization.Role) error {
	return nil
}
func (*identityAuthorizationRepo) ListPermissions(context.Context, shared.PageQuery) ([]*domainAuthorization.Permission, int64, error) {
	return nil, 0, nil
}
func (*identityAuthorizationRepo) FindPermissionsByCodes(context.Context, []string) ([]*domainAuthorization.Permission, error) {
	return nil, nil
}
func (*identityAuthorizationRepo) ListRolePermissions(context.Context, uint) ([]*domainAuthorization.Permission, error) {
	return nil, nil
}
func (*identityAuthorizationRepo) EnsureRolePermissions(context.Context, uint, []uint) error {
	return nil
}
func (*identityAuthorizationRepo) ReplaceRolePermissions(context.Context, uint, []uint) error {
	return nil
}
func (*identityAuthorizationRepo) ListUserRoles(context.Context, uint) ([]*domainAuthorization.Role, error) {
	return nil, nil
}
func (*identityAuthorizationRepo) ListUserPermissionCodes(context.Context, uint) ([]string, error) {
	return nil, nil
}
func (*identityAuthorizationRepo) ReplaceUserRoles(context.Context, uint, []uint) error { return nil }
func (*identityAuthorizationRepo) CountUsersByRoleCode(context.Context, string) (int64, error) {
	return 0, nil
}
