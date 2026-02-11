package repository

import (
	"context"
	"errors"

	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/domain"
	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/observability"
	"gorm.io/gorm"
)

var ErrRoleNotFound = errors.New("role not found")

type RoleRepository interface {
	FindByID(id uint) (*domain.Role, error)
	FindByName(name string) (*domain.Role, error)
	List() ([]domain.Role, error)
	ListPaged(req PageRequest, sortBy, sortOrder, name string) (PageResult[domain.Role], error)
	Create(role *domain.Role, permissionIDs []uint) error
	Update(role *domain.Role, permissionIDs []uint) error
	DeleteByID(id uint) error
}

type GormRoleRepository struct{ db *gorm.DB }

func NewRoleRepository(db *gorm.DB) RoleRepository { return &GormRoleRepository{db: db} }

func (r *GormRoleRepository) FindByID(id uint) (*domain.Role, error) {
	var role domain.Role
	err := r.db.Preload("Permissions").First(&role, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			observability.RecordRepositoryOperation(context.Background(), "role", "find_by_id", "not_found")
			return nil, ErrRoleNotFound
		}
		observability.RecordRepositoryOperation(context.Background(), "role", "find_by_id", "error")
		return nil, err
	}
	observability.RecordRepositoryOperation(context.Background(), "role", "find_by_id", "success")
	return &role, nil
}

func (r *GormRoleRepository) FindByName(name string) (*domain.Role, error) {
	var role domain.Role
	err := r.db.Preload("Permissions").Where("name = ?", name).First(&role).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			observability.RecordRepositoryOperation(context.Background(), "role", "find_by_name", "not_found")
			return nil, ErrRoleNotFound
		}
		observability.RecordRepositoryOperation(context.Background(), "role", "find_by_name", "error")
		return nil, err
	}
	observability.RecordRepositoryOperation(context.Background(), "role", "find_by_name", "success")
	return &role, nil
}

func (r *GormRoleRepository) List() ([]domain.Role, error) {
	var roles []domain.Role
	err := r.db.Preload("Permissions").Find(&roles).Error
	if err != nil {
		observability.RecordRepositoryOperation(context.Background(), "role", "list", "error")
		return roles, err
	}
	observability.RecordRepositoryOperation(context.Background(), "role", "list", "success")
	return roles, err
}

func (r *GormRoleRepository) ListPaged(req PageRequest, sortBy, sortOrder, name string) (PageResult[domain.Role], error) {
	normalized := normalizePageRequest(req)
	result := PageResult[domain.Role]{
		Page:     normalized.Page,
		PageSize: normalized.PageSize,
	}

	base := r.db.Model(&domain.Role{})
	if name != "" {
		base = base.Where("roles.name LIKE ?", name+"%")
	}
	if err := base.Count(&result.Total).Error; err != nil {
		observability.RecordRepositoryOperation(context.Background(), "role", "list_paged", "error")
		return PageResult[domain.Role]{}, err
	}

	query := base.Preload("Permissions")
	if sortBy != "" {
		query = query.Order("roles." + sortBy + " " + sortOrder)
	}
	query = query.Order("roles.id " + sortOrder)
	offset := (normalized.Page - 1) * normalized.PageSize
	if err := query.Offset(offset).Limit(normalized.PageSize).Find(&result.Items).Error; err != nil {
		observability.RecordRepositoryOperation(context.Background(), "role", "list_paged", "error")
		return PageResult[domain.Role]{}, err
	}
	result.TotalPages = calcTotalPages(result.Total, normalized.PageSize)
	observability.RecordRepositoryOperation(context.Background(), "role", "list_paged", "success")
	return result, nil
}

func (r *GormRoleRepository) Create(role *domain.Role, permissionIDs []uint) error {
	if err := r.db.Create(role).Error; err != nil {
		observability.RecordRepositoryOperation(context.Background(), "role", "create", "error")
		return err
	}
	if len(permissionIDs) == 0 {
		observability.RecordRepositoryOperation(context.Background(), "role", "create", "success")
		return nil
	}
	var perms []domain.Permission
	if err := r.db.Where("id IN ?", permissionIDs).Find(&perms).Error; err != nil {
		observability.RecordRepositoryOperation(context.Background(), "role", "create", "error")
		return err
	}
	if err := r.db.Model(role).Association("Permissions").Replace(perms); err != nil {
		observability.RecordRepositoryOperation(context.Background(), "role", "create", "error")
		return err
	}
	observability.RecordRepositoryOperation(context.Background(), "role", "create", "success")
	return nil
}

func (r *GormRoleRepository) Update(role *domain.Role, permissionIDs []uint) error {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var existing domain.Role
		if err := tx.Preload("Permissions").First(&existing, role.ID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRoleNotFound
			}
			return err
		}
		if err := tx.Model(&existing).Updates(map[string]any{
			"name":        role.Name,
			"description": role.Description,
		}).Error; err != nil {
			return err
		}
		var perms []domain.Permission
		if len(permissionIDs) > 0 {
			if err := tx.Where("id IN ?", permissionIDs).Find(&perms).Error; err != nil {
				return err
			}
		}
		return tx.Model(&existing).Association("Permissions").Replace(perms)
	})
	if err != nil {
		if errors.Is(err, ErrRoleNotFound) {
			observability.RecordRepositoryOperation(context.Background(), "role", "update", "not_found")
		} else {
			observability.RecordRepositoryOperation(context.Background(), "role", "update", "error")
		}
		return err
	}
	observability.RecordRepositoryOperation(context.Background(), "role", "update", "success")
	return nil
}

func (r *GormRoleRepository) DeleteByID(id uint) error {
	res := r.db.Delete(&domain.Role{}, id)
	if res.Error != nil {
		observability.RecordRepositoryOperation(context.Background(), "role", "delete_by_id", "error")
		return res.Error
	}
	if res.RowsAffected == 0 {
		observability.RecordRepositoryOperation(context.Background(), "role", "delete_by_id", "not_found")
		return ErrRoleNotFound
	}
	observability.RecordRepositoryOperation(context.Background(), "role", "delete_by_id", "success")
	return nil
}
