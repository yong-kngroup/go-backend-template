package authorization

import (
	"context"
	"errors"
	"testing"
	"time"

	domainAuthorization "github.com/freeDog-wy/go-backend-template/internal/domain/authorization"
	domainIdentity "github.com/freeDog-wy/go-backend-template/internal/domain/identity"
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
)

func TestEnsureAdminAccess(t *testing.T) {
	t.Parallel()

	t.Run("allows a user with any permission after defaults are installed", func(t *testing.T) {
		t.Parallel()
		repo := newAuthorizationRepo()
		repo.userPermissionCodes = []string{"user.read"}
		service := New(&authorizationTx{}, repo, nil, nil, nil)

		if err := service.EnsureAdminAccess(context.Background(), 7); err != nil {
			t.Fatalf("EnsureAdminAccess() error = %v", err)
		}
		if repo.defaultsInstalled {
			t.Fatal("EnsureAdminAccess() should not install default permissions")
		}
	})

	t.Run("denies a user without permissions", func(t *testing.T) {
		t.Parallel()
		service := New(&authorizationTx{}, newAuthorizationRepo(), nil, nil, nil)
		if err := service.EnsureAdminAccess(context.Background(), 7); !errors.Is(err, domainAuthorization.ErrPermissionDenied) {
			t.Fatalf("EnsureAdminAccess() error = %v", err)
		}
	})
}

func TestReplaceUserRoles(t *testing.T) {
	t.Parallel()
	user, _ := domainIdentity.NewAdminUser("Admin", "admin@example.com")
	user.AssignID(9)
	role := domainAuthorization.ReconstituteRole(2, "operator", "Operator", "", time.Now(), time.Now())
	repo := newAuthorizationRepo()
	repo.rolesByID[2] = role
	users := &authorizationUserRepo{user: user}
	bus := &authorizationBus{}
	service := New(&authorizationTx{}, repo, users, bus, nil)

	err := service.ReplaceUserRoles(context.Background(), ReplaceUserRolesCmd{UserID: 9, RoleIDs: []uint{2, 2}, ActorUserID: 1})
	if err != nil {
		t.Fatalf("ReplaceUserRoles() error = %v", err)
	}
	if len(repo.replacedRoleIDs) != 1 || repo.replacedRoleIDs[0] != 2 {
		t.Fatalf("role IDs = %v, want [2]", repo.replacedRoleIDs)
	}
	if len(bus.events) != 1 || bus.events[0].EventName() != "audit.log.requested" {
		t.Fatalf("audit events = %#v", bus.events)
	}
}

func TestUpdateRoleMapsMissingRole(t *testing.T) {
	t.Parallel()
	service := New(&authorizationTx{}, newAuthorizationRepo(), nil, nil, nil)
	_, err := service.UpdateRole(context.Background(), UpdateRoleCmd{RoleID: 404, Name: "Updated"})
	if !errors.Is(err, domainAuthorization.ErrRoleNotFound) {
		t.Fatalf("UpdateRole() error = %v", err)
	}
}

type authorizationTx struct{}

func (*authorizationTx) Do(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type authorizationRepo struct {
	rolesByID           map[uint]*domainAuthorization.Role
	userPermissionCodes []string
	defaultsInstalled   bool
	replacedRoleIDs     []uint
}

func newAuthorizationRepo() *authorizationRepo {
	return &authorizationRepo{rolesByID: make(map[uint]*domainAuthorization.Role)}
}
func (r *authorizationRepo) EnsurePermissions(context.Context, []*domainAuthorization.Permission) error {
	r.defaultsInstalled = true
	return nil
}
func (*authorizationRepo) EnsureRole(context.Context, *domainAuthorization.Role) (*domainAuthorization.Role, error) {
	return domainAuthorization.ReconstituteRole(1, domainAuthorization.SuperAdminRoleCode, "Super Admin", "", time.Now(), time.Now()), nil
}
func (*authorizationRepo) ListRoles(context.Context, shared.PageQuery) ([]*domainAuthorization.Role, int64, error) {
	return nil, 0, nil
}
func (r *authorizationRepo) FindRoleByID(_ context.Context, id uint) (*domainAuthorization.Role, error) {
	role := r.rolesByID[id]
	if role == nil {
		return nil, shared.ErrNotFound
	}
	return role, nil
}
func (*authorizationRepo) FindRoleByCode(context.Context, string) (*domainAuthorization.Role, error) {
	return nil, shared.ErrNotFound
}
func (r *authorizationRepo) FindRolesByIDs(_ context.Context, ids []uint) ([]*domainAuthorization.Role, error) {
	roles := make([]*domainAuthorization.Role, 0, len(ids))
	for _, id := range ids {
		if role := r.rolesByID[id]; role != nil {
			roles = append(roles, role)
		}
	}
	return roles, nil
}
func (*authorizationRepo) CreateRole(context.Context, *domainAuthorization.Role) error { return nil }
func (*authorizationRepo) UpdateRole(context.Context, *domainAuthorization.Role) error { return nil }
func (*authorizationRepo) ListPermissions(context.Context, shared.PageQuery) ([]*domainAuthorization.Permission, int64, error) {
	return nil, 0, nil
}
func (r *authorizationRepo) FindPermissionsByCodes(_ context.Context, codes []string) ([]*domainAuthorization.Permission, error) {
	permissions := make([]*domainAuthorization.Permission, 0, len(codes))
	for _, code := range codes {
		permission, _ := domainAuthorization.NewPermission(code, code, "")
		permissions = append(permissions, permission)
	}
	return permissions, nil
}
func (*authorizationRepo) ListRolePermissions(context.Context, uint) ([]*domainAuthorization.Permission, error) {
	return nil, nil
}
func (*authorizationRepo) EnsureRolePermissions(context.Context, uint, []uint) error  { return nil }
func (*authorizationRepo) ReplaceRolePermissions(context.Context, uint, []uint) error { return nil }
func (*authorizationRepo) ListUserRoles(context.Context, uint) ([]*domainAuthorization.Role, error) {
	return nil, nil
}
func (r *authorizationRepo) ListUserPermissionCodes(context.Context, uint) ([]string, error) {
	return r.userPermissionCodes, nil
}
func (r *authorizationRepo) ReplaceUserRoles(_ context.Context, _ uint, ids []uint) error {
	r.replacedRoleIDs = append([]uint(nil), ids...)
	return nil
}
func (*authorizationRepo) CountUsersByRoleCode(context.Context, string) (int64, error) { return 0, nil }

type authorizationUserRepo struct{ user *domainIdentity.User }

func (r *authorizationUserRepo) FindByID(context.Context, uint) (*domainIdentity.User, error) {
	if r.user == nil {
		return nil, shared.ErrNotFound
	}
	return r.user, nil
}
func (*authorizationUserRepo) FindByEmail(context.Context, string) (*domainIdentity.User, error) {
	return nil, shared.ErrNotFound
}
func (*authorizationUserRepo) List(context.Context, shared.PageQuery) ([]*domainIdentity.User, int64, error) {
	return nil, 0, nil
}
func (*authorizationUserRepo) Create(context.Context, *domainIdentity.User) error { return nil }
func (*authorizationUserRepo) Update(context.Context, *domainIdentity.User) error { return nil }
func (*authorizationUserRepo) Delete(context.Context, uint) error                 { return nil }

type authorizationBus struct{ events []shared.Event }

func (b *authorizationBus) Publish(_ context.Context, events ...shared.Event) error {
	b.events = append(b.events, events...)
	return nil
}
