package service

import (
	"errors"
	"net/http"
	"time"

	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/repository"
	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/security"
)

type SessionView struct {
	ID        uint       `json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	UserAgent string     `json:"user_agent"`
	IP        string     `json:"ip"`
	IsCurrent bool       `json:"is_current"`
}

type SessionService struct {
	sessionRepo repository.SessionRepository
	pepper      string
}

func NewSessionService(sessionRepo repository.SessionRepository, pepper string) *SessionService {
	return &SessionService{
		sessionRepo: sessionRepo,
		pepper:      pepper,
	}
}

func (s *SessionService) ListActiveSessions(userID uint, currentSessionID uint) ([]SessionView, error) {
	sessions, err := s.sessionRepo.ListActiveByUserID(userID)
	if err != nil {
		return nil, err
	}
	views := make([]SessionView, 0, len(sessions))
	for _, session := range sessions {
		views = append(views, SessionView{
			ID:        session.ID,
			CreatedAt: session.CreatedAt,
			ExpiresAt: session.ExpiresAt,
			RevokedAt: session.RevokedAt,
			UserAgent: session.UserAgent,
			IP:        session.IP,
			IsCurrent: session.ID == currentSessionID,
		})
	}
	return views, nil
}

func (s *SessionService) ResolveCurrentSessionID(r *http.Request, claims *security.Claims, userID uint) (uint, error) {
	if claims != nil && claims.ID != "" {
		session, err := s.sessionRepo.FindActiveByTokenIDForUser(userID, claims.ID)
		if err == nil {
			return session.ID, nil
		}
		if !errors.Is(err, repository.ErrSessionNotFound) {
			return 0, err
		}
	}

	refreshToken := security.GetCookie(r, "refresh_token")
	if refreshToken == "" {
		return 0, repository.ErrSessionNotFound
	}
	hash := security.HashRefreshToken(refreshToken, s.pepper)
	session, err := s.sessionRepo.FindByHash(hash)
	if err != nil {
		return 0, err
	}
	if session.UserID != userID || session.RevokedAt != nil || session.ExpiresAt.Before(time.Now()) {
		return 0, repository.ErrSessionNotFound
	}
	return session.ID, nil
}

func (s *SessionService) RevokeSession(userID, sessionID uint) (string, error) {
	changed, err := s.sessionRepo.RevokeByIDForUser(userID, sessionID, "user_session_revoked")
	if err != nil {
		return "", err
	}
	if !changed {
		return "already_revoked", nil
	}
	return "revoked", nil
}

func (s *SessionService) RevokeOtherSessions(userID, currentSessionID uint) (int64, error) {
	return s.sessionRepo.RevokeOthersByUser(userID, currentSessionID, "user_revoke_others")
}
