package authorization

import (
	"context"
	"errors"
	"time"

	domainAuthorization "github.com/freeDog-wy/go-backend-template/internal/domain/authorization"
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	"github.com/freeDog-wy/go-backend-template/internal/infra/database"
	modelAuthorization "github.com/freeDog-wy/go-backend-template/internal/model/authorization"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

var _ domainAuthorization.Repository = (*Repository)(nil)

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) EnsurePermissions(ctx context.Context, permissions []*domainAuthorization.Permission) error {
	for _, permission := range permissions {
		if permission == nil {
			continue
		}
		model := modelAuthorization.PermissionFromEntity(permission)
		if err := database.DB(ctx, r.db).
			Where("code = ?", model.Code).
			Assign(map[string]any{
				"name":        model.Name,
				"description": model.Description,
			}).
			FirstOrCreate(model).
			Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) EnsureRole(ctx context.Context, role *domainAuthorization.Role) (*domainAuthorization.Role, error) {
	model := modelAuthorization.RoleFromEntity(role)
	if err := database.DB(ctx, r.db).
		Where("code = ?", model.Code).
		Assign(map[string]any{
			"name":        model.Name,
			"description": model.Description,
		}).
		FirstOrCreate(model).
		Error; err != nil {
		return nil, err
	}
	return model.ToEntity(), nil
}

func (r *Repository) ListRoles(ctx context.Context, page shared.PageQuery) ([]*domainAuthorization.Role, int64, error) {
	var total int64
	if err := database.DB(ctx, r.db).Model(&modelAuthorization.Role{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var models []modelAuthorization.Role
	if err := database.DB(ctx, r.db).
		Order("id ASC").
		Limit(page.PerPage).
		Offset(page.Offset()).
		Find(&models).Error; err != nil {
		return nil, 0, err
	}
	roles := make([]*domainAuthorization.Role, 0, len(models))
	for i := range models {
		roles = append(roles, models[i].ToEntity())
	}
	return roles, total, nil
}

func (r *Repository) FindRoleByID(ctx context.Context, roleID uint) (*domainAuthorization.Role, error) {
	var model modelAuthorization.Role
	if err := database.DB(ctx, r.db).Where("id = ?", roleID).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, shared.ErrNotFound
		}
		return nil, err
	}
	return model.ToEntity(), nil
}

func (r *Repository) FindRoleByCode(ctx context.Context, code string) (*domainAuthorization.Role, error) {
	var model modelAuthorization.Role
	if err := database.DB(ctx, r.db).Where("code = ?", code).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, shared.ErrNotFound
		}
		return nil, err
	}
	return model.ToEntity(), nil
}

func (r *Repository) FindRolesByIDs(ctx context.Context, roleIDs []uint) ([]*domainAuthorization.Role, error) {
	if len(roleIDs) == 0 {
		return []*domainAuthorization.Role{}, nil
	}

	var models []modelAuthorization.Role
	if err := database.DB(ctx, r.db).Where("id IN ?", roleIDs).Find(&models).Error; err != nil {
		return nil, err
	}

	roles := make([]*domainAuthorization.Role, 0, len(models))
	for i := range models {
		roles = append(roles, models[i].ToEntity())
	}
	return roles, nil
}

func (r *Repository) CreateRole(ctx context.Context, role *domainAuthorization.Role) error {
	model := modelAuthorization.RoleFromEntity(role)
	if err := database.DB(ctx, r.db).Create(model).Error; err != nil {
		return err
	}
	role.AssignID(model.ID)
	return nil
}

func (r *Repository) UpdateRole(ctx context.Context, role *domainAuthorization.Role) error {
	model := modelAuthorization.RoleFromEntity(role)
	return database.DB(ctx, r.db).
		Model(&modelAuthorization.Role{}).
		Where("id = ?", model.ID).
		Updates(map[string]any{
			"name":        model.Name,
			"description": model.Description,
		}).Error
}

func (r *Repository) ListPermissions(ctx context.Context, page shared.PageQuery) ([]*domainAuthorization.Permission, int64, error) {
	var total int64
	if err := database.DB(ctx, r.db).Model(&modelAuthorization.Permission{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var models []modelAuthorization.Permission
	if err := database.DB(ctx, r.db).
		Order("code ASC").
		Limit(page.PerPage).
		Offset(page.Offset()).
		Find(&models).Error; err != nil {
		return nil, 0, err
	}
	permissions := make([]*domainAuthorization.Permission, 0, len(models))
	for i := range models {
		permissions = append(permissions, models[i].ToEntity())
	}
	return permissions, total, nil
}

func (r *Repository) FindPermissionsByCodes(ctx context.Context, codes []string) ([]*domainAuthorization.Permission, error) {
	if len(codes) == 0 {
		return []*domainAuthorization.Permission{}, nil
	}
	var models []modelAuthorization.Permission
	if err := database.DB(ctx, r.db).Where("code IN ?", codes).Find(&models).Error; err != nil {
		return nil, err
	}
	permissions := make([]*domainAuthorization.Permission, 0, len(models))
	for i := range models {
		permissions = append(permissions, models[i].ToEntity())
	}
	return permissions, nil
}

func (r *Repository) ListRolePermissions(ctx context.Context, roleID uint) ([]*domainAuthorization.Permission, error) {
	var models []modelAuthorization.Permission
	if err := database.DB(ctx, r.db).
		Table("permissions").
		Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Where("role_permissions.role_id = ?", roleID).
		Order("permissions.code ASC").
		Find(&models).Error; err != nil {
		return nil, err
	}
	permissions := make([]*domainAuthorization.Permission, 0, len(models))
	for i := range models {
		permissions = append(permissions, models[i].ToEntity())
	}
	return permissions, nil
}

func (r *Repository) EnsureRolePermissions(ctx context.Context, roleID uint, permissionIDs []uint) error {
	if len(permissionIDs) == 0 {
		return nil
	}

	now := time.Now()
	records := make([]modelAuthorization.RolePermission, 0, len(permissionIDs))
	for _, permissionID := range permissionIDs {
		records = append(records, modelAuthorization.RolePermission{
			RoleID:       roleID,
			PermissionID: permissionID,
			CreatedAt:    now,
		})
	}

	return database.DB(ctx, r.db).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "role_id"},
				{Name: "permission_id"},
			},
			DoNothing: true,
		}).
		Create(&records).Error
}

func (r *Repository) ReplaceRolePermissions(ctx context.Context, roleID uint, permissionIDs []uint) error {
	db := database.DB(ctx, r.db)
	if err := db.Where("role_id = ?", roleID).Delete(&modelAuthorization.RolePermission{}).Error; err != nil {
		return err
	}
	if len(permissionIDs) == 0 {
		return nil
	}

	now := time.Now()
	records := make([]modelAuthorization.RolePermission, 0, len(permissionIDs))
	for _, permissionID := range permissionIDs {
		records = append(records, modelAuthorization.RolePermission{
			RoleID:       roleID,
			PermissionID: permissionID,
			CreatedAt:    now,
		})
	}
	return db.Create(&records).Error
}

func (r *Repository) ListUserRoles(ctx context.Context, userID uint) ([]*domainAuthorization.Role, error) {
	var models []modelAuthorization.Role
	if err := database.DB(ctx, r.db).
		Table("roles").
		Joins("JOIN user_roles ON user_roles.role_id = roles.id").
		Where("user_roles.user_id = ?", userID).
		Order("roles.code ASC").
		Find(&models).Error; err != nil {
		return nil, err
	}
	roles := make([]*domainAuthorization.Role, 0, len(models))
	for i := range models {
		roles = append(roles, models[i].ToEntity())
	}
	return roles, nil
}

func (r *Repository) ListUserPermissionCodes(ctx context.Context, userID uint) ([]string, error) {
	var codes []string
	err := database.DB(ctx, r.db).
		Table("permissions").
		Distinct("permissions.code").
		Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Joins("JOIN user_roles ON user_roles.role_id = role_permissions.role_id").
		Where("user_roles.user_id = ?", userID).
		Order("permissions.code ASC").
		Pluck("permissions.code", &codes).Error
	return codes, err
}

func (r *Repository) ReplaceUserRoles(ctx context.Context, userID uint, roleIDs []uint) error {
	db := database.DB(ctx, r.db)
	if err := db.Where("user_id = ?", userID).Delete(&modelAuthorization.UserRole{}).Error; err != nil {
		return err
	}
	if len(roleIDs) == 0 {
		return nil
	}

	now := time.Now()
	records := make([]modelAuthorization.UserRole, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		records = append(records, modelAuthorization.UserRole{
			UserID:    userID,
			RoleID:    roleID,
			CreatedAt: now,
		})
	}
	return db.Create(&records).Error
}

func (r *Repository) CountUsersByRoleCode(ctx context.Context, roleCode string) (int64, error) {
	var count int64
	err := database.DB(ctx, r.db).
		Table("user_roles").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Where("roles.code = ?", roleCode).
		Count(&count).Error
	return count, err
}
