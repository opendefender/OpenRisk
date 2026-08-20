// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"gorm.io/gorm"
)

// MultitenantOrgService handles organization operations
type MultitenantOrgService struct {
	db *gorm.DB
}

// NewMultitenantOrgService creates a new multi-tenant organization service
func NewMultitenantOrgService(db *gorm.DB) *MultitenantOrgService {
	return &MultitenantOrgService{db: db}
}

// CreateOrgRequest is the request to create a new organization
type CreateOrgRequestMultitenant struct {
	Name     string `json:"name" validate:"required,min=1"`
	Slug     string `json:"slug" validate:"required,min=1"`
	LogoURL  string `json:"logo_url,omitempty"`
	Industry string `json:"industry,omitempty"`
	Size     string `json:"size,omitempty"`
	Plan     string `json:"plan,omitempty"`
}

// CreateOrganization creates a new organization and makes the user the root
func (s *MultitenantOrgService) CreateOrganization(ctx context.Context, req *CreateOrgRequestMultitenant, ownerID uuid.UUID) (*domain.Organization, error) {
	// Validate unique slug
	var existing domain.Organization
	result := s.db.WithContext(ctx).Where("slug = ?", req.Slug).First(&existing)
	if result.Error == nil {
		return nil, errors.New("organization slug already exists")
	}
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, result.Error
	}

	// Create organization
	org := &domain.Organization{
		ID:       uuid.New(),
		Name:     req.Name,
		Slug:     req.Slug,
		LogoURL:  req.LogoURL,
		Industry: req.Industry,
		Size:     domain.OrgSize(req.Size),
		Plan:     domain.OrgPlan(req.Plan),
		OwnerID:  ownerID,
		IsActive: true,
	}

	if err := s.db.WithContext(ctx).Create(org).Error; err != nil {
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	// Create root membership for owner
	rootMember := &domain.OrganizationMember{
		ID:             uuid.New(),
		OrganizationID: org.ID,
		UserID:         ownerID,
		Role:           domain.RoleRoot,
		IsActive:       true,
	}

	if err := s.db.WithContext(ctx).Create(rootMember).Error; err != nil {
		// Rollback organization creation
		s.db.WithContext(ctx).Delete(org)
		return nil, fmt.Errorf("failed to create root membership: %w", err)
	}

	// Seed system profiles for the organization
	if err := s.seedSystemProfiles(ctx, org.ID, ownerID); err != nil {
		return nil, fmt.Errorf("failed to seed system profiles: %w", err)
	}

	// Set as default org for user if it's their first
	var count int64
	s.db.WithContext(ctx).Model(&domain.OrganizationMember{}).
		Where("user_id = ?", ownerID).
		Count(&count)

	if count == 1 {
		s.db.WithContext(ctx).Model(&domain.User{}).
			Where("id = ?", ownerID).
			Update("default_org_id", org.ID)
	}

	return org, nil
}

// GetOrganizationByID retrieves an organization by ID
func (s *MultitenantOrgService) GetOrganizationByID(ctx context.Context, orgID uuid.UUID) (*domain.Organization, error) {
	var org domain.Organization
	if err := s.db.WithContext(ctx).
		Preload("Owner").
		Preload("Members").
		First(&org, orgID).Error; err != nil {
		return nil, err
	}
	return &org, nil
}

// GetOrganizationBySlug retrieves an organization by slug
func (s *MultitenantOrgService) GetOrganizationBySlug(ctx context.Context, slug string) (*domain.Organization, error) {
	var org domain.Organization
	if err := s.db.WithContext(ctx).
		Preload("Owner").
		Preload("Members").
		Where("slug = ?", slug).
		First(&org).Error; err != nil {
		return nil, err
	}
	return &org, nil
}

// GetUserOrganizations returns all organizations a user belongs to
func (s *MultitenantOrgService) GetUserOrganizations(ctx context.Context, userID uuid.UUID) ([]domain.Organization, error) {
	var orgs []domain.Organization
	err := s.db.WithContext(ctx).
		Joins("JOIN organization_members ON organization_members.organization_id = organizations.id").
		Where("organization_members.user_id = ? AND organization_members.is_active = ?", userID, true).
		Preload("Owner").
		Find(&orgs).Error
	return orgs, err
}

// UpdateOrganization updates an organization
func (s *MultitenantOrgService) UpdateOrganization(ctx context.Context, orgID uuid.UUID, updates map[string]interface{}) (*domain.Organization, error) {
	org := &domain.Organization{}
	if err := s.db.WithContext(ctx).Model(org).Where("id = ?", orgID).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetOrganizationByID(ctx, orgID)
}

// DeleteOrganization deletes an organization (soft delete)
func (s *MultitenantOrgService) DeleteOrganization(ctx context.Context, orgID uuid.UUID) error {
	return s.db.WithContext(ctx).Model(&domain.Organization{}).Where("id = ?", orgID).Update("is_active", false).Error
}

// TransferOwnership transfers root ownership to another user
func (s *MultitenantOrgService) TransferOwnership(ctx context.Context, orgID, currentOwnerID, newOwnerID uuid.UUID) error {
	// Check that current owner is root
	var currentMember domain.OrganizationMember
	if err := s.db.WithContext(ctx).
		Where("organization_id = ? AND user_id = ? AND role = ?", orgID, currentOwnerID, domain.RoleRoot).
		First(&currentMember).Error; err != nil {
		return errors.New("current user is not the organization root")
	}

	// Check that target user is a member
	var targetMember domain.OrganizationMember
	if err := s.db.WithContext(ctx).
		Where("organization_id = ? AND user_id = ?", orgID, newOwnerID).
		First(&targetMember).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("target user is not a member of the organization")
		}
		return err
	}

	// Start transaction
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Demote current owner to admin
		if err := tx.Model(&currentMember).Update("role", domain.RoleAdmin).Error; err != nil {
			return err
		}

		// Promote new owner to root
		if err := tx.Model(&targetMember).Update("role", domain.RoleRoot).Error; err != nil {
			return err
		}

		// Update organization owner_id
		if err := tx.Model(&domain.Organization{}).Where("id = ?", orgID).Update("owner_id", newOwnerID).Error; err != nil {
			return err
		}

		return nil
	})
}

// Invitations were served from here by a second, unwired implementation that
// stored a UUID token in clear, never checked the invitee's email on
// acceptance, and took its organization id from the URL rather than the
// session. It has been removed rather than left beside its replacement: the
// real member-invitation flow lives in internal/application/membership, backed
// by internal/domain/invitation.go.

// seedSystemProfiles creates default IAM profiles for a new organization
func (s *MultitenantOrgService) seedSystemProfiles(ctx context.Context, orgID, createdByID uuid.UUID) error {
	systemProfiles := []struct {
		name        string
		description string
		permissions []struct {
			resource domain.Resource
			action   domain.Action
			scope    domain.Scope
		}
	}{
		{
			name:        "Read Only",
			description: "View-only access to all resources",
			permissions: []struct {
				resource domain.Resource
				action   domain.Action
				scope    domain.Scope
			}{
				{domain.ResourceRisks, domain.ActionRead, domain.ScopeAll},
				{domain.ResourceAssets, domain.ActionRead, domain.ScopeAll},
				{domain.ResourceMitigations, domain.ActionRead, domain.ScopeAll},
				{domain.ResourceReports, domain.ActionRead, domain.ScopeAll},
				{domain.ResourceAuditLogs, domain.ActionRead, domain.ScopeAll},
			},
		},
		{
			name:        "Analyst",
			description: "Create and manage risks, assets, and mitigations",
			permissions: []struct {
				resource domain.Resource
				action   domain.Action
				scope    domain.Scope
			}{
				{domain.ResourceRisks, domain.ActionRead, domain.ScopeAll},
				{domain.ResourceRisks, domain.ActionWrite, domain.ScopeAll},
				{domain.ResourceAssets, domain.ActionRead, domain.ScopeAll},
				{domain.ResourceAssets, domain.ActionWrite, domain.ScopeAll},
				{domain.ResourceMitigations, domain.ActionRead, domain.ScopeAll},
				{domain.ResourceMitigations, domain.ActionWrite, domain.ScopeAll},
				{domain.ResourceReports, domain.ActionRead, domain.ScopeAll},
			},
		},
		{
			name:        "Manager",
			description: "Full access to risk management, reporting, and team management",
			permissions: []struct {
				resource domain.Resource
				action   domain.Action
				scope    domain.Scope
			}{
				{domain.ResourceRisks, domain.ActionRead, domain.ScopeAll},
				{domain.ResourceRisks, domain.ActionWrite, domain.ScopeAll},
				{domain.ResourceRisks, domain.ActionDelete, domain.ScopeAll},
				{domain.ResourceAssets, domain.ActionRead, domain.ScopeAll},
				{domain.ResourceAssets, domain.ActionWrite, domain.ScopeAll},
				{domain.ResourceAssets, domain.ActionDelete, domain.ScopeAll},
				{domain.ResourceMitigations, domain.ActionRead, domain.ScopeAll},
				{domain.ResourceMitigations, domain.ActionWrite, domain.ScopeAll},
				{domain.ResourceMitigations, domain.ActionDelete, domain.ScopeAll},
				{domain.ResourceReports, domain.ActionRead, domain.ScopeAll},
				{domain.ResourceReports, domain.ActionWrite, domain.ScopeAll},
				{domain.ResourceMembers, domain.ActionRead, domain.ScopeAll},
			},
		},
	}

	for _, profTemplate := range systemProfiles {
		profile := &domain.Profile{
			ID:             uuid.New(),
			OrganizationID: orgID,
			Name:           profTemplate.name,
			Description:    profTemplate.description,
			IsSystem:       true,
			CreatedByID:    createdByID,
		}

		if err := s.db.WithContext(ctx).Create(profile).Error; err != nil {
			return err
		}

		// Create permissions for this profile
		for _, perm := range profTemplate.permissions {
			permission := &domain.ProfilePermission{
				ID:        uuid.New(),
				ProfileID: profile.ID,
				Resource:  perm.resource,
				Action:    perm.action,
				Scope:     perm.scope,
			}
			if err := s.db.WithContext(ctx).Create(permission).Error; err != nil {
				return err
			}
		}
	}

	return nil
}
