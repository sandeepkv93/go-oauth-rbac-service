package repository

import (
	"errors"
	"time"

	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrSessionNotFound = errors.New("session not found")

type SessionRepository interface {
	Create(s *domain.Session) error
	FindByHash(hash string) (*domain.Session, error)
	FindActiveByTokenIDForUser(userID uint, tokenID string) (*domain.Session, error)
	FindByIDForUser(userID, sessionID uint) (*domain.Session, error)
	ListActiveByUserID(userID uint) ([]domain.Session, error)
	RotateSession(oldHash string, newSession *domain.Session) (*domain.Session, error)
	UpdateTokenLineageByHash(hash, tokenID, familyID string) error
	MarkReuseDetectedByHash(hash string) error
	RevokeByHash(hash, reason string) error
	RevokeByIDForUser(userID, sessionID uint, reason string) (bool, error)
	RevokeOthersByUser(userID, keepSessionID uint, reason string) (int64, error)
	RevokeByFamilyID(familyID, reason string) (int64, error)
	RevokeByUserID(userID uint, reason string) error
	CleanupExpired() (int64, error)
}

type GormSessionRepository struct{ db *gorm.DB }

func NewSessionRepository(db *gorm.DB) SessionRepository { return &GormSessionRepository{db: db} }

func (r *GormSessionRepository) Create(s *domain.Session) error { return r.db.Create(s).Error }

func (r *GormSessionRepository) FindByHash(hash string) (*domain.Session, error) {
	var s domain.Session
	err := r.db.Where("refresh_token_hash = ?", hash).First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *GormSessionRepository) FindActiveByTokenIDForUser(userID uint, tokenID string) (*domain.Session, error) {
	var s domain.Session
	err := r.db.Where("user_id = ? AND token_id = ? AND revoked_at IS NULL AND expires_at > ?", userID, tokenID, time.Now()).
		First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *GormSessionRepository) FindByIDForUser(userID, sessionID uint) (*domain.Session, error) {
	var s domain.Session
	err := r.db.Where("user_id = ? AND id = ?", userID, sessionID).First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *GormSessionRepository) ListActiveByUserID(userID uint) ([]domain.Session, error) {
	var sessions []domain.Session
	err := r.db.Where("user_id = ? AND revoked_at IS NULL AND expires_at > ?", userID, time.Now()).
		Order("created_at DESC").
		Find(&sessions).Error
	return sessions, err
}

func (r *GormSessionRepository) RotateSession(oldHash string, newSession *domain.Session) (*domain.Session, error) {
	var rotated *domain.Session
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var s domain.Session
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("refresh_token_hash = ? AND revoked_at IS NULL AND expires_at > ?", oldHash, time.Now()).
			First(&s).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrSessionNotFound
			}
			return err
		}
		now := time.Now().UTC()
		reason := "rotated"
		if err := tx.Model(&domain.Session{}).
			Where("id = ?", s.ID).
			Updates(map[string]any{"revoked_at": now, "revoked_reason": reason}).Error; err != nil {
			return err
		}
		if err := tx.Create(newSession).Error; err != nil {
			return err
		}
		s.RevokedAt = &now
		s.RevokedReason = &reason
		rotated = &s
		return nil
	})
	if err != nil {
		return nil, err
	}
	return rotated, nil
}

func (r *GormSessionRepository) UpdateTokenLineageByHash(hash, tokenID, familyID string) error {
	updates := map[string]any{
		"token_id":  tokenID,
		"family_id": familyID,
	}
	return r.db.Model(&domain.Session{}).
		Where("refresh_token_hash = ? AND (token_id IS NULL OR token_id = '' OR family_id IS NULL OR family_id = '')", hash).
		Updates(updates).Error
}

func (r *GormSessionRepository) MarkReuseDetectedByHash(hash string) error {
	now := time.Now().UTC()
	reason := "reuse_detected"
	return r.db.Model(&domain.Session{}).
		Where("refresh_token_hash = ?", hash).
		Updates(map[string]any{"reuse_detected_at": now, "revoked_reason": reason}).Error
}

func (r *GormSessionRepository) RevokeByHash(hash, reason string) error {
	now := time.Now()
	return r.db.Model(&domain.Session{}).
		Where("refresh_token_hash = ? AND revoked_at IS NULL", hash).
		Updates(map[string]any{"revoked_at": now, "revoked_reason": reason}).Error
}

func (r *GormSessionRepository) RevokeByIDForUser(userID, sessionID uint, reason string) (bool, error) {
	session, err := r.FindByIDForUser(userID, sessionID)
	if err != nil {
		return false, err
	}
	if session.RevokedAt != nil {
		return false, nil
	}
	now := time.Now().UTC()
	res := r.db.Model(&domain.Session{}).
		Where("user_id = ? AND id = ? AND revoked_at IS NULL", userID, sessionID).
		Updates(map[string]any{"revoked_at": now, "revoked_reason": reason})
	return res.RowsAffected > 0, res.Error
}

func (r *GormSessionRepository) RevokeOthersByUser(userID, keepSessionID uint, reason string) (int64, error) {
	now := time.Now().UTC()
	res := r.db.Model(&domain.Session{}).
		Where("user_id = ? AND id <> ? AND revoked_at IS NULL", userID, keepSessionID).
		Updates(map[string]any{"revoked_at": now, "revoked_reason": reason})
	return res.RowsAffected, res.Error
}

func (r *GormSessionRepository) RevokeByFamilyID(familyID, reason string) (int64, error) {
	now := time.Now().UTC()
	res := r.db.Model(&domain.Session{}).
		Where("family_id = ? AND revoked_at IS NULL", familyID).
		Updates(map[string]any{"revoked_at": now, "revoked_reason": reason})
	return res.RowsAffected, res.Error
}

func (r *GormSessionRepository) RevokeByUserID(userID uint, reason string) error {
	now := time.Now()
	return r.db.Model(&domain.Session{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Updates(map[string]any{"revoked_at": now, "revoked_reason": reason}).Error
}

func (r *GormSessionRepository) CleanupExpired() (int64, error) {
	res := r.db.Where("expires_at <= ?", time.Now()).Delete(&domain.Session{})
	return res.RowsAffected, res.Error
}
