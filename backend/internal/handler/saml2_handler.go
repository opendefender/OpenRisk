// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package handler

import (
	"fmt"
	"os"

	"github.com/gofiber/fiber/v2"
)

// SAML sign-in is turned off (#866).
//
// The assertion consumer service used to read the email out of whatever XML it
// was posted and open a session for that account, with no signature, issuer,
// audience, validity-window or InResponseTo check. Both entry points now refuse
// every request and send the browser to the login screen, which already says
// "this sign-in method is not configured". SAML comes back through a maintained
// library that verifies signed assertions, not by patching a hand-rolled parser.

// SAML2InitiateLogin refuses: there is no ACS that could finish the flow.
func SAML2InitiateLogin(c *fiber.Ctx) error {
	return oauthFailure(c, "provider_not_configured", "saml2", oauthLocale(c))
}

// SAML2ACS refuses every assertion, well-formed or not. It creates no user and
// no session.
func SAML2ACS(c *fiber.Ctx) error {
	// Get SAML Response from POST
	samlResponse := c.FormValue("SAMLResponse")
	if samlResponse == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "SAML Response not provided",
		})
	}

	// Decode base64
	decoded, err := base64.StdEncoding.DecodeString(samlResponse)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to decode SAML Response: %v", err),
		})
	}

	// Parse XML
	var response SAMLResponse
	if err := xml.Unmarshal(decoded, &response); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to parse SAML Response: %v", err),
		})
	}

	// Validate response
	if response.Status.StatusCode.Value != "urn:oasis:names:tc:SAML:2.0:status:Success" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fmt.Sprintf("SAML authentication failed: %s", response.Status.StatusCode.Value),
		})
	}

	// Extract user information from assertion
	assertion := response.Assertion
	email := assertion.Subject.NameID
	userInfo := &OAuth2UserInfo{
		Email:    email,
		Provider: "saml2",
	}

	// Extract attributes
	for _, attr := range assertion.AttributeStatement.Attributes {
		switch attr.Name {
		case "email":
			if len(attr.Values) > 0 {
				userInfo.Email = attr.Values[0].Text
			}
		case "emailAddress":
			if len(attr.Values) > 0 {
				userInfo.Email = attr.Values[0].Text
			}
		case "displayName", "name":
			if len(attr.Values) > 0 {
				userInfo.Name = attr.Values[0].Text
			}
		case "groups", "memberOf":
			for _, val := range attr.Values {
				userInfo.Groups = append(userInfo.Groups, val.Text)
			}
		}
	}

	// Use email as name if name not found
	if userInfo.Name == "" {
		userInfo.Name = strings.Split(userInfo.Email, "@")[0]
	}

	// Provision user
	user, err := provisionSAML2User(userInfo)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to provision user: %v", err),
		})
	}

	// Apply group-based role mapping if configured
	if len(userInfo.Groups) > 0 {
		if err := applyGroupRoleMapping(user, userInfo.Groups); err != nil {
			log.Printf("Warning: failed to apply group role mapping for user %s: %v", user.ID, err)
		}
	}

	// Issue an RS256 access+refresh pair via the SAME TokenManager as password
	// login (previously HS256, rejected by the RS256 middleware). Onboarding +
	// audit happen inside issueSSOSession. SAML carries no sanitised return
	// target (RelayState is not read), so the browser lands on the SPA home.
	return issueSSOSession(c, user, "saml2", "", oauthLocale(c))
}

// provisionSAML2User finds or creates a user from SAML2 assertion
func provisionSAML2User(userInfo *OAuth2UserInfo) (*domain.User, error) {
	user := &domain.User{}

	// Find existing user by email
	result := database.DB.Preload("Role").Where("email = ?", userInfo.Email).First(user)

	if result.Error == gorm.ErrRecordNotFound {
		// Check if auto-provisioning is enabled
		autoProvision := os.Getenv("SSO_AUTO_PROVISION")
		if autoProvision == "" {
			autoProvision = "true"
		}

		if autoProvision != "true" {
			return nil, fmt.Errorf("user auto-provisioning disabled")
		}

		// Get default role
		defaultRole := &domain.Role{}
		if err := database.DB.Where("name = ?", "viewer").First(defaultRole).Error; err != nil {
			return nil, fmt.Errorf("default role not found: %w", err)
		}

		// Create new user
		user = &domain.User{
			ID:       uuid.New(),
			Email:    userInfo.Email,
			Username: userInfo.Email,
			FullName: userInfo.Name,
			RoleID:   defaultRole.ID,
			IsActive: true,
		}

		if err := database.DB.Create(user).Error; err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}

		// Reload with role
		database.DB.Preload("Role").First(user)

		return user, nil
	}

	if result.Error != nil {
		return nil, result.Error
	}

	// Update existing user if auto-update is enabled
	autoUpdate := os.Getenv("SSO_AUTO_UPDATE_PROFILE")
	if autoUpdate == "" {
		autoUpdate = "true"
	}

	if autoUpdate == "true" {
		user.FullName = userInfo.Name
		database.DB.Save(user)
	}

	return user, nil
}

// applyGroupRoleMapping maps SAML groups to OpenRisk roles
func applyGroupRoleMapping(user *domain.User, groups []string) error {
	// Get role mapping from environment (simple JSON or key:value pairs)
	// Format: "admin-group:admin,analyst-group:analyst,viewer-group:viewer"
	mappingStr := os.Getenv("SSO_GROUP_ROLE_MAPPING")
	if mappingStr == "" {
		return nil // No mapping configured
	}

	// Parse mapping
	mapping := make(map[string]string)
	for _, pair := range strings.Split(mappingStr, ",") {
		parts := strings.Split(strings.TrimSpace(pair), ":")
		if len(parts) == 2 {
			mapping[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	// Check if any of the user's groups map to a role
	for _, group := range groups {
		if roleName, exists := mapping[group]; exists {
			// Find the role
			role := &domain.Role{}
			if err := database.DB.Where("name = ?", roleName).First(role).Error; err == nil {
				// Update user role
				user.RoleID = role.ID
				database.DB.Save(user)
				return nil
			}
		}
	}

	return nil
}

// SAMLMetadata generates SAML2 Service Provider metadata
func SAMLMetadata(c *fiber.Ctx) error {
	entityID := os.Getenv("SAML2_SP_ENTITY_ID")
	acsURL := os.Getenv("SAML2_ACS_URL")

	if entityID == "" || acsURL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "SAML2 not properly configured",
		})
	}

	metadata := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" 
  entityID="%s">
  <SPSSODescriptor 
    AuthnRequestsSigned="false"
    WantAssertionsSigned="false"
    protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</NameIDFormat>
    <AssertionConsumerService 
      Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST"
      Location="%s"
      index="0"
      isDefault="true"/>
  </SPSSODescriptor>
</EntityDescriptor>`, entityID, acsURL)

	c.Set("Content-Type", "application/xml")
	return c.SendString(metadata)
}
