// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package ownership

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// UserLookup resolves user ids to emails in one query. Satisfied structurally by
// repository.GormUserRepository.EmailsByIDs (already used by the asset history).
type UserLookup interface {
	EmailsByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
}

// InAppNotifier is the narrow slice of application/notification.UseCase this
// package needs. Optional everywhere: a deployment without notifications still
// assigns work.
type InAppNotifier interface {
	NotifyInApp(userID, tenantID uuid.UUID, notifType domain.NotificationType, subject, message string, resourceID *uuid.UUID, resourceType string) error
}

// Service validates ownership changes and tells people about them.
//
// Every dependency is optional and nil-safe, the same contract the rest of the
// application layer uses: a missing member repository degrades validation to
// "trust the caller" rather than failing the write, and a missing notifier just
// means nobody is told. Assigning work must not break because the notification
// centre is down.
type Service struct {
	members  MemberRepository
	users    UserLookup
	notifier InAppNotifier
}

func NewService() *Service { return &Service{} }

func (s *Service) WithMembers(m MemberRepository) *Service { s.members = m; return s }
func (s *Service) WithUsers(u UserLookup) *Service         { s.users = u; return s }
func (s *Service) WithNotifier(n InAppNotifier) *Service   { s.notifier = n; return s }

// Validate checks that every user the patch wants to assign is an ACTIVE member
// of the tenant. This is the guard that makes cross-tenant assignment
// impossible: an id from another organisation reads back as "not a member".
//
// It returns a typed validation error rather than a not-found, because from the
// caller's point of view the request body is what is wrong.
func (s *Service) Validate(ctx context.Context, tenantID uuid.UUID, patch domain.OwnershipPatch) error {
	candidates := patch.CandidateIDs()
	if len(candidates) == 0 || s.members == nil {
		return nil
	}
	for _, id := range candidates {
		member, err := s.members.GetMember(ctx, tenantID, id)
		if err != nil {
			return err
		}
		if member == nil {
			return domain.NewValidationError(fmt.Sprintf("user %s is not a member of this organization", id))
		}
		if !member.IsActive {
			return domain.NewValidationError(fmt.Sprintf("user %s is deactivated and cannot be assigned work", id))
		}
	}
	return nil
}

// Apply validates the patch, applies it to the block, and returns the changes
// that actually happened — ready to hand to Notify.
func (s *Service) Apply(ctx context.Context, tenantID uuid.UUID, block *domain.Ownership, patch domain.OwnershipPatch, actor uuid.UUID) ([]domain.OwnershipChange, error) {
	if block == nil || patch.IsEmpty() {
		return nil, nil
	}
	if err := s.Validate(ctx, tenantID, patch); err != nil {
		return nil, err
	}
	before := *block
	patch.Apply(block)
	return domain.DiffOwnership(before, *block, actor), nil
}

// ResolveEmails fills the computed *_email fields of a batch of entities in a
// single lookup. Degrades to a no-op when the lookup is unavailable — the UI
// then renders the short id instead of an address, which is honest.
func (s *Service) ResolveEmails(ctx context.Context, entities ...domain.OwnedEntity) {
	if s.users == nil || len(entities) == 0 {
		return
	}
	seen := map[uuid.UUID]bool{}
	ids := make([]uuid.UUID, 0, len(entities)*3)
	for _, e := range entities {
		if e == nil {
			continue
		}
		block := e.OwnershipBlock()
		if block == nil {
			continue
		}
		for _, id := range block.DistinctUserIDs() {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	if len(ids) == 0 {
		return
	}
	emails, err := s.users.EmailsByIDs(ctx, ids)
	if err != nil {
		return // degrade: no emails, never a failed read
	}
	for _, e := range entities {
		if e == nil {
			continue
		}
		if block := e.OwnershipBlock(); block != nil {
			block.ResolveEmails(emails)
		}
	}
}

// Notify tells each newly assigned user about their new responsibility.
//
// Best-effort by construction: errors are swallowed. Nobody's risk assignment
// should fail because a notification row could not be written — and no actor is
// notified about assigning something to themselves.
func (s *Service) Notify(_ context.Context, tenantID uuid.UUID, changes []domain.OwnershipChange, subject domain.OwnershipSubject) {
	if s.notifier == nil || len(changes) == 0 || tenantID == uuid.Nil {
		return
	}
	for _, ch := range changes {
		if ch.To == nil || *ch.To == uuid.Nil || *ch.To == ch.Actor {
			continue
		}
		title, message := assignmentCopy(ch.Role, subject)
		var resourceID *uuid.UUID
		if subject.ResourceID != uuid.Nil {
			id := subject.ResourceID
			resourceID = &id
		}
		_ = s.notifier.NotifyInApp(
			*ch.To, tenantID,
			domain.NotificationTypeActionAssigned,
			title, message,
			resourceID, subject.ResourceType,
		)
	}
}

// assignmentCopy builds the notification title/body. Kept here (not in the
// handler) so every entry point words it the same way.
func assignmentCopy(role domain.OwnershipRole, subject domain.OwnershipSubject) (string, string) {
	en := strings.HasPrefix(strings.ToLower(subject.Locale), "en")
	name := subject.Title
	if name == "" {
		name = subject.ResourceType
	}

	if en {
		switch role {
		case domain.RoleOwner:
			return "You are now the owner", fmt.Sprintf("You have been made owner of %q. You answer for its outcome.", name)
		case domain.RoleReviewer:
			return "You are now the reviewer", fmt.Sprintf("You have been asked to validate %q once the work is done.", name)
		default:
			return "New assignment", fmt.Sprintf("%q has been assigned to you.", name)
		}
	}
	switch role {
	case domain.RoleOwner:
		return "Vous êtes désormais responsable", fmt.Sprintf("Vous avez été désigné responsable de « %s ». Vous en répondez.", name)
	case domain.RoleReviewer:
		return "Vous êtes désormais validateur", fmt.Sprintf("Vous devez valider « %s » une fois le travail terminé.", name)
	default:
		return "Nouvelle affectation", fmt.Sprintf("« %s » vous a été assigné.", name)
	}
}
