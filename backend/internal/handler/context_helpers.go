// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package handler

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func safeGetUUID(c *fiber.Ctx, key string) uuid.UUID {
	val := c.Locals(key)
	if val == nil {
		return uuid.Nil
	}
	if u, ok := val.(uuid.UUID); ok {
		return u
	}
	if s, ok := val.(string); ok {
		parsed, err := uuid.Parse(s)
		if err == nil {
			return parsed
		}
	}
	return uuid.Nil
}

func safeGetString(c *fiber.Ctx, key string) string {
	val := c.Locals(key)
	if val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	if u, ok := val.(uuid.UUID); ok {
		return u.String()
	}
	return fmt.Sprintf("%v", val)
}
