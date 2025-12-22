# Backend Endpoints Implementation Guide

## Overview
This document provides a comprehensive guide to all implemented backend endpoints. All endpoints are now ready to be integrated with the frontend.

---

## 1. User Management Endpoints

### 1.1 Create User (POST /users)
**Access Level:** Admin only

**Request:**
```json
{
  "email": "newuser@example.com",
  "username": "newuser",
  "full_name": "New User",
  "password": "SecurePassword123",
  "role": "analyst",
  "department": "Security Team"
}
```

**Response (201 Created):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "newuser@example.com",
  "username": "newuser",
  "full_name": "New User",
  "role": "analyst",
  "is_active": true,
  "created_at": "2025-12-22T10:30:00Z"
}
```

**Error Cases:**
- 400: Invalid input (missing required fields, email format invalid, password < 8 chars)
- 409: Email or username already exists
- 500: Server error

---

### 1.2 Update User Profile (PATCH /users/:id)
**Access Level:** Protected (any authenticated user can update their own profile)

**Request:**
```json
{
  "full_name": "Updated Name",
  "bio": "Security specialist focused on risk management",
  "phone": "+33612345678",
  "department": "Security Operations",
  "timezone": "Europe/Paris"
}
```

**Response (200 OK):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "user@example.com",
  "username": "user",
  "full_name": "Updated Name",
  "role": "analyst",
  "is_active": true,
  "created_at": "2025-12-22T08:00:00Z"
}
```

**Features:**
- Partial update: only provided fields are updated
- Bio can be empty string
- Timezone defaults to UTC if not provided
- All fields are optional

**Error Cases:**
- 400: Invalid input format
- 401: Unauthorized (not authenticated)
- 404: User not found
- 500: Server error

---

### 1.3 Get Users (GET /users)
**Access Level:** Admin only

**Response (200 OK):**
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "admin@example.com",
    "username": "admin",
    "full_name": "Admin User",
    "role": "admin",
    "is_active": true,
    "created_at": "2025-12-22T00:00:00Z",
    "last_login": "2025-12-22T14:30:00Z"
  },
  {
    "id": "650e8400-e29b-41d4-a716-446655440001",
    "email": "analyst@example.com",
    "username": "analyst",
    "full_name": "Analyst User",
    "role": "analyst",
    "is_active": true,
    "created_at": "2025-12-22T05:00:00Z",
    "last_login": null
  }
]
```

---

### 1.4 Update User Status (PATCH /users/:id/status)
**Access Level:** Admin only

**Request:**
```json
{
  "is_active": false
}
```

**Response (200 OK):**
```json
{
  "message": "User status updated"
}
```

---

### 1.5 Update User Role (PATCH /users/:id/role)
**Access Level:** Admin only

**Request:**
```json
{
  "role": "admin"
}
```

**Response (200 OK):**
```json
{
  "message": "User role updated"
}
```

**Available Roles:**
- `viewer`: Read-only access
- `analyst`: Full read/write access to risks and incidents
- `admin`: Full system access including user management

---

### 1.6 Delete User (DELETE /users/:id)
**Access Level:** Admin only

**Response (204 No Content):**
(Empty response)

**Error Cases:**
- 400: Cannot delete own account
- 401: Unauthorized
- 403: Forbidden (not admin)
- 404: User not found

---

## 2. Team Management Endpoints

### 2.1 Create Team (POST /teams)
**Access Level:** Admin only

**Request:**
```json
{
  "name": "Security Team",
  "description": "Main security operations team"
}
```

**Response (201 Created):**
```json
{
  "id": "750e8400-e29b-41d4-a716-446655440000",
  "name": "Security Team",
  "description": "Main security operations team",
  "member_count": 0,
  "created_at": "2025-12-22T10:30:00Z"
}
```

---

### 2.2 Get Teams (GET /teams)
**Access Level:** Admin only

**Response (200 OK):**
```json
[
  {
    "id": "750e8400-e29b-41d4-a716-446655440000",
    "name": "Security Team",
    "description": "Main security operations team",
    "member_count": 5,
    "created_at": "2025-12-22T10:30:00Z"
  },
  {
    "id": "850e8400-e29b-41d4-a716-446655440001",
    "name": "Compliance Team",
    "description": "Compliance and audit team",
    "member_count": 3,
    "created_at": "2025-12-22T11:00:00Z"
  }
]
```

---

### 2.3 Get Team Details (GET /teams/:id)
**Access Level:** Admin only

**Response (200 OK):**
```json
{
  "id": "750e8400-e29b-41d4-a716-446655440000",
  "name": "Security Team",
  "description": "Main security operations team",
  "member_count": 2,
  "members": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "email": "analyst1@example.com",
      "full_name": "Analyst One",
      "role": "member",
      "joined_at": "2025-12-22T10:35:00Z"
    },
    {
      "id": "650e8400-e29b-41d4-a716-446655440001",
      "email": "analyst2@example.com",
      "full_name": "Analyst Two",
      "role": "member",
      "joined_at": "2025-12-22T11:15:00Z"
    }
  ],
  "created_at": "2025-12-22T10:30:00Z"
}
```

---

### 2.4 Update Team (PATCH /teams/:id)
**Access Level:** Admin only

**Request:**
```json
{
  "name": "Updated Team Name",
  "description": "Updated description"
}
```

**Response (200 OK):**
```json
{
  "id": "750e8400-e29b-41d4-a716-446655440000",
  "name": "Updated Team Name",
  "description": "Updated description",
  "member_count": 2,
  "created_at": "2025-12-22T10:30:00Z"
}
```

---

### 2.5 Delete Team (DELETE /teams/:id)
**Access Level:** Admin only

**Response (204 No Content):**
(Empty response - team and all member associations are deleted)

---

### 2.6 Add Team Member (POST /teams/:id/members/:userId)
**Access Level:** Admin only

**Response (200 OK):**
```json
{
  "message": "Member added to team"
}
```

**Error Cases:**
- 404: Team or user not found
- 409: User is already a member of this team
- 500: Database error

---

### 2.7 Remove Team Member (DELETE /teams/:id/members/:userId)
**Access Level:** Admin only

**Response (204 No Content):**
(Empty response)

---

## 3. Integration Testing Endpoints

### 3.1 Test Integration (POST /integrations/:id/test)
**Access Level:** Protected (any authenticated user)

**Request:**
```json
{
  "api_url": "https://api.example.com/health",
  "api_key": "sk_test_xxxxxxxxxxxx"
}
```

**Success Response (200 OK):**
```json
{
  "success": true,
  "message": "Integration test successful",
  "status": 200,
  "timestamp": "2025-12-22T10:35:00Z",
  "details": null
}
```

**Failure Response (400 Bad Request):**
```json
{
  "success": false,
  "message": "Integration test failed",
  "status": 401,
  "timestamp": "2025-12-22T10:35:00Z",
  "details": "Unauthorized"
}
```

**Features:**
- HTTP client with 10-second timeout
- Bearer token authentication
- Response body inspection (limited to 1KB for safety)
- Automatic retry logic with exponential backoff
- Comprehensive error messages
- Audit logging of test results

**Error Cases:**
- 400: Invalid API URL, failed to connect, timeout
- 401: Unauthorized
- 500: Server error

---

## 4. Database Migrations

### Applied Migrations:

#### 0008_add_user_profile_fields.sql
Adds the following fields to the `users` table:
- `bio` (TEXT)
- `phone` (VARCHAR 20)
- `department` (VARCHAR 255)
- `timezone` (VARCHAR 100, defaults to 'UTC')

#### 0009_create_teams_table.sql
Creates two new tables:

**teams:**
- `id` (UUID, PK)
- `name` (VARCHAR 255, indexed)
- `description` (TEXT)
- `metadata` (JSONB)
- `created_at` (TIMESTAMP)
- `updated_at` (TIMESTAMP)
- `deleted_at` (TIMESTAMP, soft delete)

**team_members:**
- `id` (UUID, PK)
- `team_id` (UUID, FK)
- `user_id` (UUID, FK)
- `role` (VARCHAR 50, defaults to 'member')
- `joined_at` (TIMESTAMP)
- `created_at` (TIMESTAMP)
- `updated_at` (TIMESTAMP)
- `deleted_at` (TIMESTAMP, soft delete)

---

## 5. Security & Access Control

### Authentication
- JWT-based authentication required for all protected endpoints
- Bearer token in Authorization header: `Authorization: Bearer <token>`

### Authorization
- **Public endpoints:** `/auth/*` (login, register, refresh)
- **Protected endpoints:** Requires valid JWT token
- **Admin-only endpoints:** Requires JWT + admin role

### Audit Logging
- All user creation, deletion, and role changes are logged
- Integration test attempts are logged (success/failure)
- IP address and User-Agent are captured for all actions

### Password Security
- Passwords are hashed with bcrypt (cost: 14)
- Minimum password length: 8 characters
- Passwords are never returned in API responses

---

## 6. Error Handling

All error responses follow this format:
```json
{
  "error": "Error description"
}
```

Common HTTP Status Codes:
- **200 OK:** Successful GET, PATCH, POST
- **201 Created:** Successful POST (new resource)
- **204 No Content:** Successful DELETE
- **400 Bad Request:** Invalid input or validation error
- **401 Unauthorized:** Missing or invalid authentication
- **403 Forbidden:** Authenticated but insufficient permissions
- **404 Not Found:** Resource not found
- **409 Conflict:** Resource already exists (email/username/duplicate team member)
- **500 Internal Server Error:** Server error

---

## 7. Frontend Integration Notes

### User Creation Flow
1. Admin navigates to Users page
2. Clicks "Create User" button → Opens CreateUserModal
3. Fills form: email, username, full_name, password, role, department
4. Clicks "Create"
5. Frontend sends `POST /users` with form data
6. On success: Modal closes, user list refreshes
7. On error: Toast notification with error message

### Profile Update Flow
1. User navigates to Settings → General tab
2. Clicks "Edit Profile" button
3. Updates fields: full_name, bio, phone, department, timezone
4. Clicks "Save"
5. Frontend sends `PATCH /users/:id` with updated fields
6. On success: User data refreshes, toast shows "Profile updated"
7. On error: Toast shows error message, fields retain user input

### Team Management Flow
1. Admin navigates to Settings → Team tab
2. Can create team with "Create Team" button
3. Can view team members
4. Can add/remove members from each team
5. Can delete teams

### Integration Testing Flow
1. User navigates to Settings → Integrations tab
2. Enters API URL and API Key for integration
3. Clicks "Test Connection"
4. Frontend sends `POST /integrations/:id/test`
5. Shows result: "VERIFIED" (green) or "FAILED" (red)
6. Stores last test result with timestamp

---

## 8. Testing the Endpoints

### Using cURL

**Create User:**
```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "username": "testuser",
    "full_name": "Test User",
    "password": "TestPass123",
    "role": "analyst"
  }'
```

**Update Profile:**
```bash
curl -X PATCH http://localhost:8080/api/v1/users/<user-id> \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "bio": "Security specialist",
    "phone": "+33612345678",
    "timezone": "Europe/Paris"
  }'
```

**Create Team:**
```bash
curl -X POST http://localhost:8080/api/v1/teams \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Security Team",
    "description": "Main security team"
  }'
```

**Test Integration:**
```bash
curl -X POST http://localhost:8080/api/v1/integrations/test-id/test \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "api_url": "https://api.example.com/health",
    "api_key": "sk_test_xxxx"
  }'
```

---

## 9. Deployment Checklist

- [ ] Run migrations: `go run ./cmd/server/main.go` (auto-migrations enabled)
- [ ] Verify database tables created successfully
- [ ] Test endpoints with postman or curl
- [ ] Verify audit logs are being recorded
- [ ] Test JWT token generation and validation
- [ ] Verify password hashing works correctly
- [ ] Test error handling for edge cases
- [ ] Verify CORS configuration for frontend domain
- [ ] Set `JWT_SECRET` environment variable in production
- [ ] Enable HTTPS in production
- [ ] Configure database connection string
- [ ] Set `APP_ENV=production` for strict CORS

---

## 10. Known Limitations & Future Improvements

### Current Limitations
1. Team metadata (JSONB field) is not yet utilized in endpoints
2. Team member roles (owner, manager, member) are set but not enforced in authorization
3. No pagination for large user/team lists
4. Integration test retries use fixed exponential backoff

### Planned Enhancements
1. Implement permission system based on team member roles
2. Add pagination and filtering to GET endpoints
3. Add team-based resource sharing
4. Implement real-time notifications for team changes
5. Add bulk user import/export functionality
6. Implement advanced audit log filtering and search

---

**Last Updated:** December 22, 2025
**Status:** ✅ All endpoints implemented and tested
