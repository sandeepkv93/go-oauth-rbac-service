package repository

import (
	"context"
	"errors"

	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/domain"
	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/observability"

	"gorm.io/gorm"
)

type UserListQuery struct {
	PageRequest
	SortBy    string
	SortOrder string
	Email     string
	Status    string
	Role      string
}

type UserRepository interface {
	FindByID(id uint) (*domain.User, error)
	FindByEmail(email string) (*domain.User, error)
	Create(user *domain.User) error
	Update(user *domain.User) error
	List() ([]domain.User, error)
	ListPaged(query UserListQuery) (PageResult[domain.User], error)
	SetRoles(userID uint, roleIDs []uint) error
	AddRole(userID, roleID uint) error
}

type GormUserRepository struct{ db *gorm.DB }

func NewUserRepository(db *gorm.DB) UserRepository { return &GormUserRepository{db: db} }

func (r *GormUserRepository) FindByID(id uint) (*domain.User, error) {
	var u domain.User
	err := r.db.Preload("Roles.Permissions").First(&u, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			observability.RecordRepositoryOperation(context.Background(), "user", "find_by_id", "not_found")
		} else {
			observability.RecordRepositoryOperation(context.Background(), "user", "find_by_id", "error")
		}
		return nil, err
	}
	observability.RecordRepositoryOperation(context.Background(), "user", "find_by_id", "success")
	return &u, nil
}

func (r *GormUserRepository) FindByEmail(email string) (*domain.User, error) {
	var u domain.User
	err := r.db.Preload("Roles.Permissions").Where("email = ?", email).First(&u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			observability.RecordRepositoryOperation(context.Background(), "user", "find_by_email", "not_found")
		} else {
			observability.RecordRepositoryOperation(context.Background(), "user", "find_by_email", "error")
		}
		return nil, err
	}
	observability.RecordRepositoryOperation(context.Background(), "user", "find_by_email", "success")
	return &u, nil
}

func (r *GormUserRepository) Create(user *domain.User) error {
	err := r.db.Create(user).Error
	if err != nil {
		observability.RecordRepositoryOperation(context.Background(), "user", "create", "error")
		return err
	}
	observability.RecordRepositoryOperation(context.Background(), "user", "create", "success")
	return nil
}
func (r *GormUserRepository) Update(user *domain.User) error {
	err := r.db.Save(user).Error
	if err != nil {
		observability.RecordRepositoryOperation(context.Background(), "user", "update", "error")
		return err
	}
	observability.RecordRepositoryOperation(context.Background(), "user", "update", "success")
	return nil
}

func (r *GormUserRepository) List() ([]domain.User, error) {
	var users []domain.User
	err := r.db.Preload("Roles").Find(&users).Error
	if err != nil {
		observability.RecordRepositoryOperation(context.Background(), "user", "list", "error")
		return users, err
	}
	observability.RecordRepositoryOperation(context.Background(), "user", "list", "success")
	return users, err
}

func (r *GormUserRepository) ListPaged(query UserListQuery) (PageResult[domain.User], error) {
	req := normalizePageRequest(query.PageRequest)
	result := PageResult[domain.User]{
		Page:     req.Page,
		PageSize: req.PageSize,
	}

	base := r.db.Model(&domain.User{})
	if query.Email != "" {
		base = base.Where("users.email LIKE ?", query.Email+"%")
	}
	if query.Status != "" {
		base = base.Where("users.status = ?", query.Status)
	}
	if query.Role != "" {
		base = base.Joins("JOIN user_roles ur ON ur.user_id = users.id").
			Joins("JOIN roles r ON r.id = ur.role_id").
			Where("r.name = ?", query.Role)
	}

	countQuery := base.Session(&gorm.Session{})
	if query.Role != "" {
		countQuery = countQuery.Distinct("users.id")
	}
	if err := countQuery.Count(&result.Total).Error; err != nil {
		observability.RecordRepositoryOperation(context.Background(), "user", "list_paged", "error")
		return PageResult[domain.User]{}, err
	}

	listQuery := base.Preload("Roles")
	if query.Role != "" {
		listQuery = listQuery.Distinct("users.*")
	}
	if query.SortBy != "" {
		listQuery = listQuery.Order("users." + query.SortBy + " " + query.SortOrder)
	}
	listQuery = listQuery.Order("users.id " + query.SortOrder)

	offset := (req.Page - 1) * req.PageSize
	if err := listQuery.Offset(offset).Limit(req.PageSize).Find(&result.Items).Error; err != nil {
		observability.RecordRepositoryOperation(context.Background(), "user", "list_paged", "error")
		return PageResult[domain.User]{}, err
	}
	result.TotalPages = calcTotalPages(result.Total, req.PageSize)
	observability.RecordRepositoryOperation(context.Background(), "user", "list_paged", "success")
	return result, nil
}

func (r *GormUserRepository) SetRoles(userID uint, roleIDs []uint) error {
	var roles []domain.Role
	if len(roleIDs) > 0 {
		if err := r.db.Where("id IN ?", roleIDs).Find(&roles).Error; err != nil {
			observability.RecordRepositoryOperation(context.Background(), "user", "set_roles", "error")
			return err
		}
	}
	u := domain.User{ID: userID}
	if err := r.db.Model(&u).Association("Roles").Replace(roles); err != nil {
		observability.RecordRepositoryOperation(context.Background(), "user", "set_roles", "error")
		return err
	}
	observability.RecordRepositoryOperation(context.Background(), "user", "set_roles", "success")
	return nil
}

func (r *GormUserRepository) AddRole(userID, roleID uint) error {
	u := domain.User{ID: userID}
	role := domain.Role{ID: roleID}
	if err := r.db.Model(&u).Association("Roles").Append(&role); err != nil {
		observability.RecordRepositoryOperation(context.Background(), "user", "add_role", "error")
		return err
	}
	observability.RecordRepositoryOperation(context.Background(), "user", "add_role", "success")
	return nil
}
