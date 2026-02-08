package service

import "go-oauth-rbac-service/internal/domain"

type AuthServiceInterface interface {
	GoogleLoginURL(state string) string
	LoginWithGoogleCode(code, ua, ip string) (*LoginResult, error)
	Refresh(refreshToken, ua, ip string) (*LoginResult, error)
	Logout(userID uint) error
	ParseUserID(subject string) (uint, error)
}

type UserServiceInterface interface {
	GetByID(id uint) (*domain.User, []string, error)
	List() ([]domain.User, error)
	SetRoles(userID uint, roleIDs []uint) error
}

type RBACAuthorizer interface {
	HasPermission(permissions []string, required string) bool
}
