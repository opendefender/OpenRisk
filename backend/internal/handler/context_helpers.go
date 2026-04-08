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
