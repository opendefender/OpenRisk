package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// OrganizationRepository defines organization database operations
type OrganizationRepository interface {
	CreateOrganization(ctx context.Context, org *domain.Organization) error
	GetOrganizationByID(ctx context.Context, orgID uuid.UUID) (*domain.Organization, error)
	GetOrganizationBySlug(ctx context.Context, slug string) (*domain.Organization, error)
	GetOrganizationsByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Organization, error)
	UpdateOrganization(ctx context.Context, org *domain.Organization) error
	DeleteOrganization(ctx context.Context, orgID uuid.UUID) error
	ListOrganizations(ctx context.Context, limit, offset int) ([]domain.Organization, error)
}

// ProfileRepository defines profile database operations
type ProfileRepository interface {
	CreateProfile(ctx context.Context, profile *domain.Profile) error
	GetProfileByID(ctx context.Context, profileID uuid.UUID) (*domain.Profile, error)
	GetProfilesByOrganization(ctx context.Context, orgID uuid.UUID) ([]domain.Profile, error)
	UpdateProfile(ctx context.Context, profile *domain.Profile) error
	DeleteProfile(ctx context.Context, profileID uuid.UUID) error
	GetSystemProfiles(ctx context.Context, orgID uuid.UUID) ([]domain.Profile, error)
}

// OrganizationMemberRepository defines organization member database operations
type OrganizationMemberRepository interface {
	CreateMember(ctx context.Context, member *domain.OrganizationMember) error
	GetMemberByID(ctx context.Context, memberID uuid.UUID) (*domain.OrganizationMember, error)
	GetMemberByUserOrg(ctx context.Context, userID, orgID uuid.UUID) (*domain.OrganizationMember, error)
	GetMembersByOrganization(ctx context.Context, orgID uuid.UUID) ([]domain.OrganizationMember, error)
	GetMembersByUser(ctx context.Context, userID uuid.UUID) ([]domain.OrganizationMember, error)
	UpdateMember(ctx context.Context, member *domain.OrganizationMember) error
	DeleteMember(ctx context.Context, memberID uuid.UUID) error
	CountMembersByOrganization(ctx context.Context, orgID uuid.UUID) (int64, error)
}

// InvitationRepository defines invitation database operations
type InvitationRepository interface {
	CreateInvitation(ctx context.Context, invitation *domain.Invitation) error
	GetInvitationByToken(ctx context.Context, token uuid.UUID) (*domain.Invitation, error)
	GetInvitationsByOrganization(ctx context.Context, orgID uuid.UUID) ([]domain.Invitation, error)
	GetInvitationsByEmail(ctx context.Context, email string) ([]domain.Invitation, error)
	UpdateInvitation(ctx context.Context, invitation *domain.Invitation) error
	DeleteInvitation(ctx context.Context, invitationID uuid.UUID) error
}

// UserSessionRepository defines user session database operations
type UserSessionRepository interface {
	CreateSession(ctx context.Context, session *domain.UserSession) error
	GetSessionByTokenHash(ctx context.Context, tokenHash string) (*domain.UserSession, error)
	GetSessionsByUser(ctx context.Context, userID uuid.UUID) ([]domain.UserSession, error)
	DeleteSession(ctx context.Context, sessionID uuid.UUID) error
	DeleteExpiredSessions(ctx context.Context) error
}

// AuditLogRepository defines audit log database operations
type AuditLogRepository interface {
	CreateAuditLog(ctx context.Context, log *domain.AuditLog) error
	GetAuditLogsByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]domain.AuditLog, error)
	GetAuditLogsByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.AuditLog, error)
}
