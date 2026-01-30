package handlers

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/middleware"
)

type TestIntegrationInput struct {
	APIUrl string json:"api_url" validate:"required,url"
	APIKey string json:"api_key" validate:"required"
}

type IntegrationTestResponse struct {
	Success   bool        json:"success"
	Message   string      json:"message"
	Status    int         json:"status"
	Timestamp string      json:"timestamp"
	Details   interface{} json:"details,omitempty"
}

// TestIntegration tests an integration connection (protected route)
func TestIntegration(c fiber.Ctx) error {
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	integrationID := c.Params("id")

	// Validate UUID format
	if _, err := uuid.Parse(integrationID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid integration ID"})
	}

	input := new(TestIntegrationInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout:   time.Second,
	}

	// Create test request
	req, err := http.NewRequest("GET", input.APIUrl, nil)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(IntegrationTestResponse{
			Success:   false,
			Message:   "Invalid API URL",
			Status:    ,
			Timestamp: time.Now().Format("--T::Z"),
			Details:   err.Error(),
		})
	}

	// Add authentication header
	req.Header.Set("Authorization", "Bearer "+input.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "OpenRisk/.")

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(IntegrationTestResponse{
			Success:   false,
			Message:   "Failed to connect to API",
			Status:    ,
			Timestamp: time.Now().Format("--T::Z"),
			Details:   err.Error(),
		})
	}
	defer resp.Body.Close()

	// Read response body (limit to KB for safety)
	body, err := io.ReadAll(io.LimitReader(resp.Body, ))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(IntegrationTestResponse{
			Success:   false,
			Message:   "Failed to read response",
			Status:    resp.StatusCode,
			Timestamp: time.Now().Format("--T::Z"),
		})
	}

	// Check if status code indicates success
	success := resp.StatusCode >=  && resp.StatusCode < 

	response := IntegrationTestResponse{
		Success:   success,
		Status:    resp.StatusCode,
		Timestamp: time.Now().Format("--T::Z"),
	}

	if success {
		response.Message = "Integration test successful"
		// Log audit
		_ = auditService.LogAction(&domain.AuditLog{
			UserID:    &claims.ID,
			Action:    domain.ActionIntegrationTest,
			Resource:  domain.ResourceIntegration,
			Result:    domain.ResultSuccess,
			IPAddress: parseIPAddressHelper(c.IP()),
			UserAgent: c.Get("User-Agent"),
		})
	} else {
		response.Message = "Integration test failed"
		response.Details = string(body)
		// Log audit failure
		_ = auditService.LogAction(&domain.AuditLog{
			UserID:    &claims.ID,
			Action:    domain.ActionIntegrationTest,
			Resource:  domain.ResourceIntegration,
			Result:    domain.ResultFailure,
			IPAddress: parseIPAddressHelper(c.IP()),
			UserAgent: c.Get("User-Agent"),
		})
	}

	statusCode := fiber.StatusOK
	if !success {
		statusCode = fiber.StatusBadRequest
	}

	return c.Status(statusCode).JSON(response)
}

// TestIntegrationAdvanced performs an advanced integration test with retries and validation
func TestIntegrationAdvanced(c fiber.Ctx) error {
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	integrationID := c.Params("id")

	// Validate UUID format
	if _, err := uuid.Parse(integrationID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid integration ID"})
	}

	input := new(TestIntegrationInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	// Test connection with retry logic
	maxRetries := 
	var lastErr error
	var resp http.Response

	client := &http.Client{
		Timeout:   time.Second,
	}

	for attempt := ; attempt < maxRetries; attempt++ {
		req, err := http.NewRequest("GET", input.APIUrl, nil)
		if err != nil {
			lastErr = err
			time.Sleep(time.Second  time.Duration(attempt+))
			continue
		}

		req.Header.Set("Authorization", "Bearer "+input.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "OpenRisk/.")

		resp, err = client.Do(req)
		if err == nil {
			break
		}
		lastErr = err
		time.Sleep(time.Second  time.Duration(attempt+))
	}

	if resp == nil {
		return c.Status(fiber.StatusBadRequest).JSON(IntegrationTestResponse{
			Success:   false,
			Message:   "Failed to connect after retries",
			Status:    ,
			Timestamp: time.Now().Format("--T::Z"),
			Details:   lastErr.Error(),
		})
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(io.LimitReader(resp.Body, ))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(IntegrationTestResponse{
			Success:   false,
			Message:   "Failed to read response",
			Status:    resp.StatusCode,
			Timestamp: time.Now().Format("--T::Z"),
		})
	}

	success := resp.StatusCode >=  && resp.StatusCode < 
	message := "Integration test successful"
	if !success {
		message = "Integration test failed"
	}

	// Check if response is valid JSON
	var details interface{}
	if !bytes.HasPrefix(bytes.TrimSpace(body), []byte("null")) && len(body) >  {
		details = string(body)
	}

	return c.Status(fiber.StatusOK).JSON(IntegrationTestResponse{
		Success:   success,
		Message:   message,
		Status:    resp.StatusCode,
		Timestamp: time.Now().Format("--T::Z"),
		Details:   details,
	})
}
