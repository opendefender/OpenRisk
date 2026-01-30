package handlers

import (
	"encoding/base"
	"encoding/xml"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/services"
)

// SAMLAssertion represents a SAML assertion
type SAMLAssertion struct {
	XMLName            xml.Name               xml:"urn:oasis:names:tc:SAML:.:assertion Assertion"
	ID                 string                 xml:"ID,attr"
	Version            string                 xml:"Version,attr"
	IssueInstant       string                 xml:"IssueInstant,attr"
	Subject            SAMLSubject            xml:"urn:oasis:names:tc:SAML:.:assertion Subject"
	Issuer             SAMLIssuer             xml:"urn:oasis:names:tc:SAML:.:assertion Issuer"
	Conditions         SAMLConditions         xml:"urn:oasis:names:tc:SAML:.:assertion Conditions"
	AttributeStatement SAMLAttributeStatement xml:"urn:oasis:names:tc:SAML:.:assertion AttributeStatement"
	AuthnStatement     SAMLAuthnStatement     xml:"urn:oasis:names:tc:SAML:.:assertion AuthnStatement"
}

type SAMLSubject struct {
	NameID              string                  xml:"urn:oasis:names:tc:SAML:.:assertion NameID"
	SubjectConfirmation SAMLSubjectConfirmation xml:"urn:oasis:names:tc:SAML:.:assertion SubjectConfirmation"
}

type SAMLSubjectConfirmation struct {
	Method                  string                      xml:"Method,attr"
	SubjectConfirmationData SAMLSubjectConfirmationData xml:"urn:oasis:names:tc:SAML:.:assertion SubjectConfirmationData"
}

type SAMLSubjectConfirmationData struct {
	NotOnOrAfter string xml:"NotOnOrAfter,attr"
	Recipient    string xml:"Recipient,attr"
}

type SAMLIssuer struct {
	Format string xml:"Format,attr"
	Text   string xml:",chardata"
}

type SAMLConditions struct {
	NotBefore    string xml:"NotBefore,attr"
	NotOnOrAfter string xml:"NotOnOrAfter,attr"
}

type SAMLAttributeStatement struct {
	Attributes []SAMLAttribute xml:"urn:oasis:names:tc:SAML:.:assertion Attribute"
}

type SAMLAttribute struct {
	Name   string               xml:"Name,attr"
	Values []SAMLAttributeValue xml:"urn:oasis:names:tc:SAML:.:assertion AttributeValue"
}

type SAMLAttributeValue struct {
	Text string xml:",chardata"
}

type SAMLAuthnStatement struct {
	AuthnInstant string           xml:"AuthnInstant,attr"
	SessionIndex string           xml:"SessionIndex,attr"
	AuthnContext SAMLAuthnContext xml:"urn:oasis:names:tc:SAML:.:assertion AuthnContext"
}

type SAMLAuthnContext struct {
	AuthnContextClassRef string xml:"urn:oasis:names:tc:SAML:.:assertion AuthnContextClassRef"
}

// SAMLResponse represents a SAML Response
type SAMLResponse struct {
	XMLName      xml.Name      xml:"urn:oasis:names:tc:SAML:.:protocol Response"
	ID           string        xml:"ID,attr"
	Version      string        xml:"Version,attr"
	IssueInstant string        xml:"IssueInstant,attr"
	Destination  string        xml:"Destination,attr"
	InResponseTo string        xml:"InResponseTo,attr"
	Status       SAMLStatus    xml:"urn:oasis:names:tc:SAML:.:protocol Status"
	Assertion    SAMLAssertion xml:"urn:oasis:names:tc:SAML:.:assertion Assertion"
}

type SAMLStatus struct {
	StatusCode SAMLStatusCode xml:"urn:oasis:names:tc:SAML:.:protocol StatusCode"
}

type SAMLStatusCode struct {
	Value string xml:"Value,attr"
}

// SAMLInitiateLogin initiates SAML login flow
func SAMLInitiateLogin(c fiber.Ctx) error {
	idpURL := os.Getenv("SAML_IDP_URL")
	entityID := os.Getenv("SAML_SP_ENTITY_ID")
	acsURL := os.Getenv("SAML_ACS_URL")

	if idpURL == "" || entityID == "" || acsURL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "SAML not properly configured",
		})
	}

	// Generate AuthnRequest
	requestID := uuid.New().String()
	now := time.Now().UTC()

	// Build simple AuthnRequest (in production, use a proper SAML library)
	authRequest := fmt.Sprintf(<?xml version="." encoding="UTF-"?>
<samlp:AuthnRequest 
  xmlns:samlp="urn:oasis:names:tc:SAML:.:protocol"
  xmlns:saml="urn:oasis:names:tc:SAML:.:assertion"
  ID="%s"
  Version="."
  IssueInstant="%s"
  Destination="%s/app/login"
  AssertionConsumerServiceURL="%s"
  ProtocolBinding="urn:oasis:names:tc:SAML:.:bindings:HTTP-POST">
  <saml:Issuer>%s</saml:Issuer>
  <samlp:NameIDPolicy 
    Format="urn:oasis:names:tc:SAML:.:nameid-format:emailAddress"
    AllowCreate="true"/>
  <samlp:RequestedAuthnContext Comparison="exact">
    <saml:AuthnContextClassRef>urn:oasis:names:tc:SAML:.:ac:classes:Password</saml:AuthnContextClassRef>
  </samlp:RequestedAuthnContext>
</samlp:AuthnRequest>, requestID, now.Format("--T::Z"), idpURL, acsURL, entityID)

	// Encode request
	encodedRequest := base.StdEncoding.EncodeToString([]byte(authRequest))

	// Build redirect URL
	redirectURL := fmt.Sprintf("%s/app/login?SAMLRequest=%s", idpURL, encodedRequest)

	return c.JSON(fiber.Map{
		"redirect_url": redirectURL,
		"request_id":   requestID,
	})
}

// SAMLACS handles SAML Assertion Consumer Service (callback)
func SAMLACS(c fiber.Ctx) error {
	// Get SAML Response from POST
	samlResponse := c.FormValue("SAMLResponse")
	if samlResponse == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "SAML Response not provided",
		})
	}

	// Decode base
	decoded, err := base.StdEncoding.DecodeString(samlResponse)
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
	if response.Status.StatusCode.Value != "urn:oasis:names:tc:SAML:.:status:Success" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fmt.Sprintf("SAML authentication failed: %s", response.Status.StatusCode.Value),
		})
	}

	// Extract user information from assertion
	assertion := response.Assertion
	email := assertion.Subject.NameID
	userInfo := &OAuthUserInfo{
		Email:    email,
		Provider: "saml",
	}

	// Extract attributes
	for _, attr := range assertion.AttributeStatement.Attributes {
		switch attr.Name {
		case "email":
			if len(attr.Values) >  {
				userInfo.Email = attr.Values[].Text
			}
		case "emailAddress":
			if len(attr.Values) >  {
				userInfo.Email = attr.Values[].Text
			}
		case "displayName", "name":
			if len(attr.Values) >  {
				userInfo.Name = attr.Values[].Text
			}
		case "groups", "memberOf":
			for _, val := range attr.Values {
				userInfo.Groups = append(userInfo.Groups, val.Text)
			}
		}
	}

	// Use email as name if name not found
	if userInfo.Name == "" {
		userInfo.Name = strings.Split(userInfo.Email, "@")[]
	}

	// Provision user
	user, err := provisionSAMLUser(userInfo)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to provision user: %v", err),
		})
	}

	// Apply group-based role mapping if configured
	if len(userInfo.Groups) >  {
		applyGroupRoleMapping(user, userInfo.Groups)
	}

	// Generate JWT token
	authService := services.NewAuthService(os.Getenv("JWT_SECRET"), time.Hour)
	jwtToken, err := authService.GenerateToken(user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate token",
		})
	}

	// Log successful authentication
	auditService := services.NewAuditService()
	auditService.LogLogin(user.ID, domain.ResultSuccess, c.IP(), c.Get("User-Agent"), "")

	// Return token to frontend
	return c.JSON(fiber.Map{
		"token":    jwtToken,
		"user":     user,
		"provider": "saml",
	})
}

// provisionSAMLUser finds or creates a user from SAML assertion
func provisionSAMLUser(userInfo OAuthUserInfo) (domain.User, error) {
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
func applyGroupRoleMapping(user domain.User, groups []string) error {
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
		if len(parts) ==  {
			mapping[strings.TrimSpace(parts[])] = strings.TrimSpace(parts[])
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

// SAMLMetadata generates SAML Service Provider metadata
func SAMLMetadata(c fiber.Ctx) error {
	entityID := os.Getenv("SAML_SP_ENTITY_ID")
	acsURL := os.Getenv("SAML_ACS_URL")

	if entityID == "" || acsURL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "SAML not properly configured",
		})
	}

	metadata := fmt.Sprintf(<?xml version="." encoding="UTF-"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:.:metadata" 
  entityID="%s">
  <SPSSODescriptor 
    AuthnRequestsSigned="false"
    WantAssertionsSigned="false"
    protocolSupportEnumeration="urn:oasis:names:tc:SAML:.:protocol">
    <NameIDFormat>urn:oasis:names:tc:SAML:.:nameid-format:emailAddress</NameIDFormat>
    <AssertionConsumerService 
      Binding="urn:oasis:names:tc:SAML:.:bindings:HTTP-POST"
      Location="%s"
      index=""
      isDefault="true"/>
  </SPSSODescriptor>
</EntityDescriptor>, entityID, acsURL)

	c.Set("Content-Type", "application/xml")
	return c.SendString(metadata)
}
