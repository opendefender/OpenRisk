 Backend Endpoints Implementation Guide

 Overview
This document provides a comprehensive guide to all implemented backend endpoints. All endpoints are now ready to be integrated with the frontend.

---

 . User Management Endpoints

 . Create User (POST /users)
Access Level: Admin only

Request:
json
{
  "email": "newuser@example.com",
  "username": "newuser",
  "full_name": "New User",
  "password": "SecurePassword",
  "role": "analyst",
  "department": "Security Team"
}


Response ( Created):
json
{
  "id": "e-eb-d-a-",
  "email": "newuser@example.com",
  "username": "newuser",
  "full_name": "New User",
  "role": "analyst",
  "is_active": true,
  "created_at": "--T::Z"
}


Error Cases:
- : Invalid input (missing required fields, email format invalid, password <  chars)
- : Email or username already exists
- : Server error

---

 . Update User Profile (PATCH /users/:id)
Access Level: Protected (any authenticated user can update their own profile)

Request:
json
{
  "full_name": "Updated Name",
  "bio": "Security specialist focused on risk management",
  "phone": "+",
  "department": "Security Operations",
  "timezone": "Europe/Paris"
}


Response ( OK):
json
{
  "id": "e-eb-d-a-",
  "email": "user@example.com",
  "username": "user",
  "full_name": "Updated Name",
  "role": "analyst",
  "is_active": true,
  "created_at": "--T::Z"
}


Features:
- Partial update: only provided fields are updated
- Bio can be empty string
- Timezone defaults to UTC if not provided
- All fields are optional

Error Cases:
- : Invalid input format
- : Unauthorized (not authenticated)
- : User not found
- : Server error

---

 . Get Users (GET /users)
Access Level: Admin only

Response ( OK):
json
[
  {
    "id": "e-eb-d-a-",
    "email": "admin@example.com",
    "username": "admin",
    "full_name": "Admin User",
    "role": "admin",
    "is_active": true,
    "created_at": "--T::Z",
    "last_login": "--T::Z"
  },
  {
    "id": "e-eb-d-a-",
    "email": "analyst@example.com",
    "username": "analyst",
    "full_name": "Analyst User",
    "role": "analyst",
    "is_active": true,
    "created_at": "--T::Z",
    "last_login": null
  }
]


---

 . Update User Status (PATCH /users/:id/status)
Access Level: Admin only

Request:
json
{
  "is_active": false
}


Response ( OK):
json
{
  "message": "User status updated"
}


---

 . Update User Role (PATCH /users/:id/role)
Access Level: Admin only

Request:
json
{
  "role": "admin"
}


Response ( OK):
json
{
  "message": "User role updated"
}


Available Roles:
- viewer: Read-only access
- analyst: Full read/write access to risks and incidents
- admin: Full system access including user management

---

 . Delete User (DELETE /users/:id)
Access Level: Admin only

Response ( No Content):
(Empty response)

Error Cases:
- : Cannot delete own account
- : Unauthorized
- : Forbidden (not admin)
- : User not found

---

 . Team Management Endpoints

 . Create Team (POST /teams)
Access Level: Admin only

Request:
json
{
  "name": "Security Team",
  "description": "Main security operations team"
}


Response ( Created):
json
{
  "id": "e-eb-d-a-",
  "name": "Security Team",
  "description": "Main security operations team",
  "member_count": ,
  "created_at": "--T::Z"
}


---

 . Get Teams (GET /teams)
Access Level: Admin only

Response ( OK):
json
[
  {
    "id": "e-eb-d-a-",
    "name": "Security Team",
    "description": "Main security operations team",
    "member_count": ,
    "created_at": "--T::Z"
  },
  {
    "id": "e-eb-d-a-",
    "name": "Compliance Team",
    "description": "Compliance and audit team",
    "member_count": ,
    "created_at": "--T::Z"
  }
]


---

 . Get Team Details (GET /teams/:id)
Access Level: Admin only

Response ( OK):
json
{
  "id": "e-eb-d-a-",
  "name": "Security Team",
  "description": "Main security operations team",
  "member_count": ,
  "members": [
    {
      "id": "e-eb-d-a-",
      "email": "analyst@example.com",
      "full_name": "Analyst One",
      "role": "member",
      "joined_at": "--T::Z"
    },
    {
      "id": "e-eb-d-a-",
      "email": "analyst@example.com",
      "full_name": "Analyst Two",
      "role": "member",
      "joined_at": "--T::Z"
    }
  ],
  "created_at": "--T::Z"
}


---

 . Update Team (PATCH /teams/:id)
Access Level: Admin only

Request:
json
{
  "name": "Updated Team Name",
  "description": "Updated description"
}


Response ( OK):
json
{
  "id": "e-eb-d-a-",
  "name": "Updated Team Name",
  "description": "Updated description",
  "member_count": ,
  "created_at": "--T::Z"
}


---

 . Delete Team (DELETE /teams/:id)
Access Level: Admin only

Response ( No Content):
(Empty response - team and all member associations are deleted)

---

 . Add Team Member (POST /teams/:id/members/:userId)
Access Level: Admin only

Response ( OK):
json
{
  "message": "Member added to team"
}


Error Cases:
- : Team or user not found
- : User is already a member of this team
- : Database error

---

 . Remove Team Member (DELETE /teams/:id/members/:userId)
Access Level: Admin only

Response ( No Content):
(Empty response)

---

 . Integration Testing Endpoints

 . Test Integration (POST /integrations/:id/test)
Access Level: Protected (any authenticated user)

Request:
json
{
  "api_url": "https://api.example.com/health",
  "api_key": "sk_test_xxxxxxxxxxxx"
}


Success Response ( OK):
json
{
  "success": true,
  "message": "Integration test successful",
  "status": ,
  "timestamp": "--T::Z",
  "details": null
}


Failure Response ( Bad Request):
json
{
  "success": false,
  "message": "Integration test failed",
  "status": ,
  "timestamp": "--T::Z",
  "details": "Unauthorized"
}


Features:
- HTTP client with -second timeout
- Bearer token authentication
- Response body inspection (limited to KB for safety)
- Automatic retry logic with exponential backoff
- Comprehensive error messages
- Audit logging of test results

Error Cases:
- : Invalid API URL, failed to connect, timeout
- : Unauthorized
- : Server error

---

 . Database Migrations

 Applied Migrations:

 _add_user_profile_fields.sql
Adds the following fields to the users table:
- bio (TEXT)
- phone (VARCHAR )
- department (VARCHAR )
- timezone (VARCHAR , defaults to 'UTC')

 _create_teams_table.sql
Creates two new tables:

teams:
- id (UUID, PK)
- name (VARCHAR , indexed)
- description (TEXT)
- metadata (JSONB)
- created_at (TIMESTAMP)
- updated_at (TIMESTAMP)
- deleted_at (TIMESTAMP, soft delete)

team_members:
- id (UUID, PK)
- team_id (UUID, FK)
- user_id (UUID, FK)
- role (VARCHAR , defaults to 'member')
- joined_at (TIMESTAMP)
- created_at (TIMESTAMP)
- updated_at (TIMESTAMP)
- deleted_at (TIMESTAMP, soft delete)

---

 . Security & Access Control

 Authentication
- JWT-based authentication required for all protected endpoints
- Bearer token in Authorization header: Authorization: Bearer <token>

 Authorization
- Public endpoints: /auth/ (login, register, refresh)
- Protected endpoints: Requires valid JWT token
- Admin-only endpoints: Requires JWT + admin role

 Audit Logging
- All user creation, deletion, and role changes are logged
- Integration test attempts are logged (success/failure)
- IP address and User-Agent are captured for all actions

 Password Security
- Passwords are hashed with bcrypt (cost: )
- Minimum password length:  characters
- Passwords are never returned in API responses

---

 . Error Handling

All error responses follow this format:
json
{
  "error": "Error description"
}


Common HTTP Status Codes:
-  OK: Successful GET, PATCH, POST
-  Created: Successful POST (new resource)
-  No Content: Successful DELETE
-  Bad Request: Invalid input or validation error
-  Unauthorized: Missing or invalid authentication
-  Forbidden: Authenticated but insufficient permissions
-  Not Found: Resource not found
-  Conflict: Resource already exists (email/username/duplicate team member)
-  Internal Server Error: Server error

---

 . Frontend Integration Notes

 User Creation Flow
. Admin navigates to Users page
. Clicks "Create User" button → Opens CreateUserModal
. Fills form: email, username, full_name, password, role, department
. Clicks "Create"
. Frontend sends POST /users with form data
. On success: Modal closes, user list refreshes
. On error: Toast notification with error message

 Profile Update Flow
. User navigates to Settings → General tab
. Clicks "Edit Profile" button
. Updates fields: full_name, bio, phone, department, timezone
. Clicks "Save"
. Frontend sends PATCH /users/:id with updated fields
. On success: User data refreshes, toast shows "Profile updated"
. On error: Toast shows error message, fields retain user input

 Team Management Flow
. Admin navigates to Settings → Team tab
. Can create team with "Create Team" button
. Can view team members
. Can add/remove members from each team
. Can delete teams

 Integration Testing Flow
. User navigates to Settings → Integrations tab
. Enters API URL and API Key for integration
. Clicks "Test Connection"
. Frontend sends POST /integrations/:id/test
. Shows result: "VERIFIED" (green) or "FAILED" (red)
. Stores last test result with timestamp

---

 . Testing the Endpoints

 Using cURL

Create User:
bash
curl -X POST http://localhost:/api/v/users \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "username": "testuser",
    "full_name": "Test User",
    "password": "TestPass",
    "role": "analyst"
  }'


Update Profile:
bash
curl -X PATCH http://localhost:/api/v/users/<user-id> \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "bio": "Security specialist",
    "phone": "+",
    "timezone": "Europe/Paris"
  }'


Create Team:
bash
curl -X POST http://localhost:/api/v/teams \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Security Team",
    "description": "Main security team"
  }'


Test Integration:
bash
curl -X POST http://localhost:/api/v/integrations/test-id/test \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "api_url": "https://api.example.com/health",
    "api_key": "sk_test_xxxx"
  }'


---

 . Deployment Checklist

- [ ] Run migrations: go run ./cmd/server/main.go (auto-migrations enabled)
- [ ] Verify database tables created successfully
- [ ] Test endpoints with postman or curl
- [ ] Verify audit logs are being recorded
- [ ] Test JWT token generation and validation
- [ ] Verify password hashing works correctly
- [ ] Test error handling for edge cases
- [ ] Verify CORS configuration for frontend domain
- [ ] Set JWT_SECRET environment variable in production
- [ ] Enable HTTPS in production
- [ ] Configure database connection string
- [ ] Set APP_ENV=production for strict CORS

---

 . Known Limitations & Future Improvements

 Current Limitations
. Team metadata (JSONB field) is not yet utilized in endpoints
. Team member roles (owner, manager, member) are set but not enforced in authorization
. No pagination for large user/team lists
. Integration test retries use fixed exponential backoff

 Planned Enhancements
. Implement permission system based on team member roles
. Add pagination and filtering to GET endpoints
. Add team-based resource sharing
. Implement real-time notifications for team changes
. Add bulk user import/export functionality
. Implement advanced audit log filtering and search

---

Last Updated: December , 
Status:  All endpoints implemented and tested
