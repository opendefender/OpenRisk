# Notification System - Integration Testing Guide

## Overview

This document provides comprehensive guidance on testing the notification system across both backend (Go) and frontend (React/TypeScript).

## Backend Testing

### Test Structure

```
backend/
├── internal/
│   ├── handlers/
│   │   └── notification_handler_test.go          (API endpoint tests)
│   ├── services/
│   │   └── notification_service_test.go          (Business logic tests)
│   └── providers/
│       └── providers_test.go                     (Provider implementation tests)
└── tests/
    └── integration/
        └── notification_integration_test.go      (E2E tests)
```

### Running Backend Tests

```bash
# Run all tests
go test ./...

# Run specific test file
go test ./backend/internal/handlers -v

# Run with coverage
go test ./... -cover

# Run with detailed coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run tests matching pattern
go test ./... -run TestNotificationHandler

# Run benchmarks
go test ./... -bench=. -benchmem
```

### Backend Test Categories

#### 1. Handler Tests (notification_handler_test.go)

Tests the API endpoint layer:
- `TestGetNotifications` - Retrieve user notifications
- `TestGetUnreadCount` - Get unread notification count
- `TestMarkAsRead` - Mark single notification as read
- `TestMarkAllAsRead` - Mark all notifications as read
- `TestDeleteNotification` - Delete a notification
- `TestGetUserPreferences` - Retrieve user preferences
- `TestUpdatePreferences` - Update notification preferences
- `TestNotificationTypes` - Verify notification types
- `TestNotificationChannels` - Verify notification channels
- `TestNotificationStatuses` - Verify notification statuses
- `TestNotificationJSONMarshaling` - JSON serialization

**Key Validations:**
- Authorization header validation
- Request parameter validation
- User/tenant isolation
- Response status codes
- Error handling

#### 2. Service Tests (notification_service_test.go)

Tests the business logic layer:
- `TestCreateMitigationDeadlineNotification` - Create deadline notifications
- `TestCreateCriticalRiskNotification` - Create critical risk alerts
- `TestCreateActionAssignedNotification` - Create action assignments
- `TestNotificationStatusTransitions` - Status workflow validation
- `TestCreateNotificationPreference` - Create user preferences
- `TestCreateNotificationTemplate` - Create reusable templates
- `TestCreateNotificationLog` - Create delivery logs
- `TestMultiTenantIsolation` - Tenant data isolation
- `TestBulkNotificationCreation` - Batch operations
- `TestNotificationMetadata` - Custom metadata storage
- `TestUnreadCountCalculation` - Unread count logic
- `TestErrorHandling` - Error scenarios

**Key Validations:**
- Notification type validation
- Status transition logic
- User preference management
- Template rendering
- Metadata handling
- Multi-tenancy enforcement

#### 3. Provider Tests (providers_test.go)

Tests the notification delivery providers:
- `TestEmailProviderConfiguration` - Email SMTP setup
- `TestEmailProviderValidation` - Email config validation
- `TestSlackProviderConfiguration` - Slack webhook setup
- `TestSlackMessageFormatting` - Slack message structure
- `TestWebhookProviderConfiguration` - Webhook configuration
- `TestHMACSHA256Signing` - Webhook signature generation
- `TestWebhookSignatureVerification` - Signature validation
- `TestExponentialBackoffCalculation` - Retry logic
- `TestFailedDeliveryLog` - Error tracking
- `TestWebhookRetryLogic` - Retry mechanisms
- `TestColorCodingForNotificationTypes` - Color formatting
- `TestBulkNotificationSending` - Batch delivery
- `TestProviderErrorHandling` - Error scenarios
- `TestWebhookPayloadValidation` - Payload validation

**Key Validations:**
- Provider configuration
- Message formatting
- Signature generation/verification
- Retry mechanisms
- Error handling
- Payload validation

### Test Data Setup

```go
// Create mock user and tenant
userID := uuid.New()
tenantID := uuid.New()

// Create test notification
notif := &domain.Notification{
    ID:       uuid.New(),
    UserID:   userID,
    TenantID: tenantID,
    Type:     domain.NotificationTypeCriticalRisk,
    Channel:  domain.NotificationChannelEmail,
    Status:   domain.NotificationStatusPending,
    Subject:  "Test Critical Risk",
    Message:  "A critical risk has been detected",
    Metadata: map[string]interface{}{
        "risk_id": "123",
        "severity": "CRITICAL",
    },
}
```

### Mocking Strategies

```go
// Mock Database
type MockNotificationDB struct {
    notifications []*domain.Notification
    preferences   []*domain.NotificationPreference
}

// Mock Service
type MockNotificationService struct {
    // Mock methods
}

// Mock HTTP Client
func mockHTTPClient() *http.Client {
    // Return mock client
}
```

## Frontend Testing

### Test Structure

```
frontend/
├── src/
│   ├── components/
│   │   └── notifications/
│   │       ├── NotificationBadge.tsx
│   │       ├── NotificationCenter.tsx
│   │       ├── NotificationPreferences.tsx
│   │       └── __tests__/
│   │           └── notifications.test.tsx
│   └── setupTests.ts
├── jest.config.js
└── package.json
```

### Running Frontend Tests

```bash
# Install dependencies
npm install

# Run all tests
npm test

# Run tests in watch mode
npm test -- --watch

# Run tests with coverage
npm test -- --coverage

# Run specific test file
npm test -- NotificationBadge

# Run tests matching pattern
npm test -- --testNamePattern="should render"

# Run tests in CI mode (no watch)
npm test -- --ci --coverage
```

### Frontend Test Categories

#### 1. Component Tests

**NotificationBadge Component:**
- `should render with unread count` - Display count correctly
- `should display 99+ for large counts` - Format large numbers
- `should not display badge when unread count is 0` - Hide when empty
- `should call onClick handler when clicked` - Handle clicks
- `should animate count changes` - Animate updates
- `should render bell icon` - Display icon correctly

**NotificationCenter Component:**
- `should render notification list` - Display notifications
- `should handle loading state` - Show loading UI
- `should handle errors` - Display error messages
- `should mark notification as read` - Mark as read functionality
- `should delete notification` - Delete functionality
- `should load more notifications` - Pagination support
- `should close when onClose is called` - Close functionality
- `should not render when isOpen is false` - Conditional rendering
- `should display notification type icons` - Icon rendering

**NotificationPreferences Component:**
- `should render preferences form` - Display form
- `should toggle email notifications` - Email checkbox toggle
- `should toggle slack notifications` - Slack checkbox toggle
- `should save preferences` - Save to API
- `should test email notification` - Test functionality
- `should display success message on save` - Success feedback
- `should display error message on save failure` - Error feedback
- `should handle deadline advance days input` - Number input
- `should cancel without saving` - Cancel functionality

#### 2. Integration Tests

- `should handle authentication token correctly` - Auth header validation
- `should handle API errors gracefully` - Error handling
- `should handle network timeouts` - Timeout handling
- `should verify Bearer token in headers` - Token format validation
- `should retry on connection failures` - Retry logic
- `should handle 401 Unauthorized` - Auth error handling
- `should handle 500 Server Error` - Server error handling

### Frontend Test Setup

```typescript
// Mock axios
jest.mock('axios');

// Create mock data
const mockNotifications = [
  {
    id: '1',
    type: 'critical_risk',
    subject: 'Critical Risk Alert',
    message: 'A critical risk has been detected',
    status: 'pending',
    created_at: new Date().toISOString(),
  },
];

// Setup mock response
(axios.get as jest.Mock).mockResolvedValueOnce({
  data: { data: mockNotifications },
});
```

### Testing Patterns

```typescript
// Test async operations
it('should load notifications', async () => {
  render(<NotificationCenter isOpen={true} />);
  
  await waitFor(() => {
    expect(screen.getByText('Critical Risk')).toBeInTheDocument();
  });
});

// Test user interactions
it('should mark as read', async () => {
  render(<NotificationCenter isOpen={true} />);
  
  const button = screen.getByText('Mark as Read');
  fireEvent.click(button);
  
  await waitFor(() => {
    expect(axios.patch).toHaveBeenCalled();
  });
});

// Test error handling
it('should handle errors', async () => {
  (axios.get as jest.Mock).mockRejectedValueOnce(
    new Error('Failed to fetch')
  );
  
  render(<NotificationCenter isOpen={true} />);
  
  await waitFor(() => {
    expect(screen.getByText(/error/i)).toBeInTheDocument();
  });
});
```

## CI/CD Integration

### GitHub Actions Workflow

```yaml
name: Run Tests

on: [push, pull_request]

jobs:
  backend-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v2
        with:
          go-version: 1.19
      - run: go test ./... -v -cover

  frontend-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-node@v2
        with:
          node-version: 18
      - run: npm ci
      - run: npm test -- --coverage
      - uses: codecov/codecov-action@v2
```

## Test Coverage Goals

| Component | Target Coverage | Current |
|-----------|-----------------|---------|
| Backend Handlers | 90% | - |
| Backend Services | 90% | - |
| Backend Providers | 85% | - |
| Frontend Components | 85% | - |
| Integration | 80% | - |

## Running Full Test Suite

```bash
# Backend tests
cd backend && go test ./... -v -cover

# Frontend tests
cd frontend && npm test -- --coverage

# Integration tests
# Run both backend and frontend, verify API integration

# Generate coverage reports
# Backend: go tool cover -html=coverage.out
# Frontend: coverage/lcov.info
```

## Debugging Tests

### Backend Debug

```bash
# Run single test with verbose output
go test -v -run TestGetNotifications ./backend/internal/handlers

# Run with race detector
go test -race ./...

# Print debug info
go test -v -args -logtostderr=true
```

### Frontend Debug

```bash
# Run tests in watch mode for development
npm test -- --watch

# Debug specific component
npm test -- --testNamePattern="NotificationCenter"

# Enable debug output
DEBUG=* npm test

# Run with debugger
node --inspect-brk node_modules/.bin/jest --runInBand
```

## Performance Testing

### Backend Performance

```bash
# Run benchmarks
go test -bench=. -benchmem ./backend/internal/services

# Compare benchmarks
go test -bench=. -benchmem -benchtime=10s ./backend/internal/services
```

### Frontend Performance

```bash
# Measure rendering performance
npm test -- --testNamePattern="render"

# Memory profiling
npm test -- --detectMemoryLeaks
```

## Test Reporting

### Coverage Reports

**Backend:**
```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

**Frontend:**
```bash
npm test -- --coverage
# Reports available in coverage/
```

### Test Metrics

Track these metrics:
- Total tests: 50+ tests
- Coverage: 85%+ code coverage
- Duration: <5 minutes full suite
- Flakiness: 0% test failures
- Performance: <100ms per unit test

## Continuous Testing Strategy

1. **Unit Tests**: Run on every commit (5 min)
2. **Integration Tests**: Run on PR creation (10 min)
3. **E2E Tests**: Run on merge to main (20 min)
4. **Performance Tests**: Run nightly

## Troubleshooting

### Common Issues

**Backend Tests Failing:**
- Check UUID generation
- Verify database mock setup
- Validate time.Time comparisons
- Check JSON marshaling

**Frontend Tests Failing:**
- Clear node_modules and reinstall
- Check mock.clearAllMocks()
- Verify async/await usage
- Check DOM queries

### Getting Help

- Run tests with `-v` for verbose output
- Check test file comments for setup
- Review mock implementations
- Check error messages for details

## Next Steps

1. Integrate with CI/CD pipeline
2. Set up code coverage tracking
3. Add performance benchmarks
4. Implement E2E testing with Playwright
5. Add mutation testing for test quality

---

**Last Updated:** 2024
**Version:** 1.0
**Status:** Complete
