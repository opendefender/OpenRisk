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
	return oauthFailure(c, "provider_not_configured", "saml2", oauthLocale(c))
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
