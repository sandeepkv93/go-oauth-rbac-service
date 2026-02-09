package service

import (
	"errors"
	"strconv"
	"time"

	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/domain"
	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/repository"
	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/security"
)

type TokenService struct {
	jwtMgr      *security.JWTManager
	sessionRepo repository.SessionRepository
	pepper      string
	accessTTL   time.Duration
	refreshTTL  time.Duration
}

var (
	ErrInvalidRefreshToken       = errors.New("invalid refresh token")
	ErrRefreshTokenReuseDetected = errors.New("refresh token reuse detected")
)

func NewTokenService(jwtMgr *security.JWTManager, sessionRepo repository.SessionRepository, pepper string, accessTTL, refreshTTL time.Duration) *TokenService {
	return &TokenService{jwtMgr: jwtMgr, sessionRepo: sessionRepo, pepper: pepper, accessTTL: accessTTL, refreshTTL: refreshTTL}
}

func (s *TokenService) Issue(user *domain.User, permissions []string, ua, ip string) (access string, refresh string, csrf string, err error) {
	access, refresh, refreshClaims, csrf, err := s.mintTokenPair(user, permissions)
	if err != nil {
		return "", "", "", err
	}
	tokenID := refreshClaims.ID
	familyID := tokenID
	hash := security.HashRefreshToken(refresh, s.pepper)
	if err := s.sessionRepo.Create(&domain.Session{
		UserID:           user.ID,
		RefreshTokenHash: hash,
		TokenID:          ptr(tokenID),
		FamilyID:         ptr(familyID),
		ParentTokenID:    nil,
		UserAgent:        ua,
		IP:               ip,
		ExpiresAt:        time.Now().Add(s.refreshTTL),
	}); err != nil {
		return "", "", "", err
	}
	return access, refresh, csrf, nil
}

func (s *TokenService) Rotate(refreshToken string, userFetcher func(id uint) (*domain.User, []string, error), ua, ip string) (access string, newRefresh string, csrf string, userID uint, err error) {
	claims, err := s.jwtMgr.ParseRefreshToken(refreshToken)
	if err != nil {
		return "", "", "", 0, ErrInvalidRefreshToken
	}
	hash := security.HashRefreshToken(refreshToken, s.pepper)
	session, err := s.sessionRepo.FindByHash(hash)
	if err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			return "", "", "", 0, ErrInvalidRefreshToken
		}
		return "", "", "", 0, err
	}
	id64, err := strconv.ParseUint(claims.Subject, 10, 64)
	if err != nil {
		return "", "", "", 0, ErrInvalidRefreshToken
	}
	userID = uint(id64)
	if session.UserID != userID {
		return "", "", "", 0, ErrInvalidRefreshToken
	}
	tokenID := getString(session.TokenID)
	familyID := getString(session.FamilyID)
	if tokenID == "" || familyID == "" {
		fallbackFamilyID := claims.ID
		if fallbackFamilyID == "" {
			fallbackFamilyID = "legacy-session"
		}
		if err := s.sessionRepo.UpdateTokenLineageByHash(hash, claims.ID, fallbackFamilyID); err != nil {
			return "", "", "", 0, err
		}
		session.TokenID = ptr(claims.ID)
		session.FamilyID = ptr(fallbackFamilyID)
		tokenID = claims.ID
		familyID = fallbackFamilyID
	}
	if tokenID != "" && claims.ID != "" && tokenID != claims.ID {
		return "", "", "", 0, ErrInvalidRefreshToken
	}
	if session.ExpiresAt.Before(time.Now()) {
		return "", "", "", 0, ErrInvalidRefreshToken
	}
	if session.RevokedAt != nil {
		reason := getString(session.RevokedReason)
		if reason == "" || reason == "rotated" || reason == "reuse_detected" {
			_ = s.sessionRepo.MarkReuseDetectedByHash(hash)
			if familyID != "" {
				_, _ = s.sessionRepo.RevokeByFamilyID(familyID, "reuse_detected")
			}
			return "", "", "", 0, ErrRefreshTokenReuseDetected
		}
		return "", "", "", 0, ErrInvalidRefreshToken
	}
	user, perms, err := userFetcher(userID)
	if err != nil {
		return "", "", "", 0, err
	}
	access, newRefresh, newClaims, csrf, err := s.mintTokenPair(user, perms)
	if err != nil {
		return "", "", "", 0, err
	}
	newHash := security.HashRefreshToken(newRefresh, s.pepper)
	_, err = s.sessionRepo.RotateSession(hash, &domain.Session{
		UserID:           userID,
		RefreshTokenHash: newHash,
		TokenID:          ptr(newClaims.ID),
		FamilyID:         ptr(familyID),
		ParentTokenID:    ptr(tokenID),
		UserAgent:        ua,
		IP:               ip,
		ExpiresAt:        time.Now().Add(s.refreshTTL),
	})
	if err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			return "", "", "", 0, ErrInvalidRefreshToken
		}
		return "", "", "", 0, err
	}
	return access, newRefresh, csrf, userID, nil
}

func (s *TokenService) RevokeAll(userID uint, reason string) error {
	return s.sessionRepo.RevokeByUserID(userID, reason)
}

func (s *TokenService) mintTokenPair(user *domain.User, permissions []string) (access string, refresh string, refreshClaims *security.Claims, csrf string, err error) {
	roles := make([]string, 0, len(user.Roles))
	for _, r := range user.Roles {
		roles = append(roles, r.Name)
	}
	refresh, err = s.jwtMgr.SignRefreshToken(user.ID, s.refreshTTL)
	if err != nil {
		return "", "", nil, "", err
	}
	refreshClaims, err = s.jwtMgr.ParseRefreshToken(refresh)
	if err != nil {
		return "", "", nil, "", err
	}
	access, err = s.jwtMgr.SignAccessTokenWithJTI(user.ID, roles, permissions, s.accessTTL, refreshClaims.ID)
	if err != nil {
		return "", "", nil, "", err
	}
	csrf, err = security.NewCSRFToken()
	if err != nil {
		return "", "", nil, "", err
	}
	return access, refresh, refreshClaims, csrf, nil
}

func ptr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func getString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
