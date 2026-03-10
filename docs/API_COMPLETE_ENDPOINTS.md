# Complete API Endpoints Reference

**Version**: 1.0  
**Last Updated**: March 10, 2026  
**Total Endpoints**: 90+  

---

## API Quick Links

- **Base URL**: `https://api.openrisk.io/api/v1`
- **Auth Header**: `Authorization: Bearer <token>`
- **Content-Type**: `application/json`
- **Response**: JSON

---

## Health & Status

### Health Check
- **Endpoint**: `GET /health`
- **Auth**: ❌ No
- **Description**: Check API status
- **Response**: `{ "status": "UP", "version": "1.0.0", "db": "CONNECTED" }`

---

## Authentication (7 endpoints)

### 1. Login
- **Endpoint**: `POST /auth/login`
- **Auth**: ❌ No
- **Body**: `{ "email": "user@example.com", "password": "..." }`
- **Response**: `{ "access_token": "...", "refresh_token": "...", "expires_in": 259200 }`

### 2. Register
- **Endpoint**: `POST /auth/register`
- **Auth**: ❌ No
- **Body**: `{ "email": "user@example.com", "password": "...", "first_name": "..." }`
- **Response**: `{ "id": "uuid", "email": "...", "created_at": "..." }`

### 3. Refresh Token
- **Endpoint**: `POST /auth/refresh`
- **Auth**: ❌ No
- **Body**: `{ "refresh_token": "..." }`
- **Response**: `{ "access_token": "...", "expires_in": 259200 }`

### 4. OAuth2 Login
- **Endpoint**: `GET /auth/oauth2/login/:provider`
- **Auth**: ❌ No
- **Params**: `:provider` = google, github, microsoft, okta
- **Response**: Redirects to provider login

### 5. OAuth2 Callback
- **Endpoint**: `GET /auth/oauth2/callback/:provider`
- **Auth**: ❌ No
- **Query**: `code=...&state=...`
- **Response**: Redirects with JWT token or error

### 6. SAML2 Login
- **Endpoint**: `GET /auth/saml2/login`
- **Auth**: ❌ No
- **Response**: Redirects to SAML IdP

### 7. SAML2 ACS
- **Endpoint**: `POST /auth/saml2/acs`
- **Auth**: ❌ No
- **Body**: SAML response from IdP
- **Response**: Redirects with JWT token

### 8. SAML2 Metadata
- **Endpoint**: `GET /auth/saml2/metadata`
- **Auth**: ❌ No
- **Response**: XML SAML metadata

### 9. Get Current User Profile
- **Endpoint**: `GET /users/me`
- **Auth**: ✅ Yes
- **Response**: `{ "id": "uuid", "email": "...", "role": "...", "created_at": "..." }`

---

## Risks CRUD (5 endpoints)

### 1. List Risks
- **Endpoint**: `GET /risks`
- **Auth**: ✅ Yes (viewer+)
- **Query Params**: 
  - `page`: int (default: 1)
  - `limit`: int (default: 10)
  - `sort_by`: string (default: -created_at)
  - `status`: string (draft, open, closed)
  - `impact`: int (1-5)
- **Response**: `[ { "id": "uuid", "title": "...", "impact": 5, "probability": 4, ... } ]`

### 2. Create Risk
- **Endpoint**: `POST /risks`
- **Auth**: ✅ Yes (analyst+)
- **Body**:
  ```json
  {
    "title": "Risk title",
    "description": "Description",
    "impact": 5,
    "probability": 4,
    "tags": ["tag1", "tag2"],
    "asset_ids": ["uuid1", "uuid2"],
    "frameworks": ["ISO_27001", "CIS"]
  }
  ```
- **Response**: `{ "id": "uuid", "title": "...", ... }`

### 3. Get Risk
- **Endpoint**: `GET /risks/:id`
- **Auth**: ✅ Yes (viewer+)
- **Response**: `{ "id": "uuid", "title": "...", "status": "open", ... }`

### 4. Update Risk
- **Endpoint**: `PATCH /risks/:id`
- **Auth**: ✅ Yes (analyst+)
- **Body**: Any of `{ title, description, impact, probability, status, tags, asset_ids, frameworks }`
- **Response**: `{ "id": "uuid", ... }`

### 5. Delete Risk
- **Endpoint**: `DELETE /risks/:id`
- **Auth**: ✅ Yes (admin+)
- **Response**: `204 No Content`

---

## Mitigations (8 endpoints)

### 1. Add Mitigation to Risk
- **Endpoint**: `POST /risks/:id/mitigations`
- **Auth**: ✅ Yes (analyst+)
- **Body**:
  ```json
  {
    "title": "Apply security patch",
    "assignee": "user@example.com",
    "status": "PLANNED",
    "due_date": "2026-03-31T23:59:59Z",
    "cost": 2,
    "mitigation_time": 4
  }
  ```
- **Response**: `{ "id": "uuid", "title": "...", ... }`

### 2. Update Mitigation
- **Endpoint**: `PATCH /mitigations/:mitigationId`
- **Auth**: ✅ Yes (analyst+)
- **Body**: Any of `{ title, assignee, status, due_date, cost, progress }`
- **Response**: `{ "id": "uuid", ... }`

### 3. Toggle Mitigation Status
- **Endpoint**: `PATCH /mitigations/:mitigationId/toggle`
- **Auth**: ✅ Yes (analyst+)
- **Response**: `{ "id": "uuid", "status": "DONE", ... }`

### 4. Get Recommended Mitigations
- **Endpoint**: `GET /mitigations/recommended`
- **Auth**: ✅ Yes
- **Response**:
  ```json
  [
    {
      "risk_id": "uuid",
      "mitigation": "Description",
      "effort_days": 2,
      "spp_score": 0.95
    }
  ]
  ```

### 5. Create Sub-Action
- **Endpoint**: `POST /mitigations/:id/subactions`
- **Auth**: ✅ Yes (analyst+)
- **Body**: `{ "title": "Sub-action title" }`
- **Response**: `{ "id": "uuid", "title": "...", "completed": false }`

### 6. Toggle Sub-Action
- **Endpoint**: `PATCH /mitigations/:id/subactions/:subactionId/toggle`
- **Auth**: ✅ Yes (analyst+)
- **Response**: `{ "id": "uuid", "completed": true }`

### 7. Delete Sub-Action
- **Endpoint**: `DELETE /mitigations/:id/subactions/:subactionId`
- **Auth**: ✅ Yes (analyst+)
- **Response**: `204 No Content`

---

## Assets (2 endpoints)

### 1. List Assets
- **Endpoint**: `GET /assets`
- **Auth**: ✅ Yes (viewer+)
- **Response**: `[ { "id": "uuid", "name": "...", "type": "...", ... } ]`

### 2. Create Asset
- **Endpoint**: `POST /assets`
- **Auth**: ✅ Yes (analyst+)
- **Body**:
  ```json
  {
    "name": "Asset name",
    "type": "database|server|application|network",
    "description": "...",
    "location": "...",
    "owner": "user@example.com"
  }
  ```
- **Response**: `{ "id": "uuid", ... }`

---

## Statistics & Dashboard (15+ endpoints)

### 1. Dashboard Stats
- **Endpoint**: `GET /stats`
- **Auth**: ✅ Yes (cached)
- **Response**: `{ "total_risks": 45, "open_risks": 12, "critical_severity_risks": 1, ... }`

### 2. Risk Matrix
- **Endpoint**: `GET /stats/risk-matrix`
- **Auth**: ✅ Yes (cached)
- **Response**: `{ "matrix": [[0,2,5],[1,4,8],...], "total_risks": 45 }`

### 3. Risk Distribution
- **Endpoint**: `GET /stats/risk-distribution`
- **Auth**: ✅ Yes (cached)
- **Response**: `{ "by_status": {...}, "by_severity": {...} }`

### 4. Mitigation Metrics
- **Endpoint**: `GET /stats/mitigation-metrics`
- **Auth**: ✅ Yes (cached)
- **Response**: `{ "total": 50, "completed": 20, "in_progress": 15, "planned": 15 }`

### 5. Top Vulnerabilities
- **Endpoint**: `GET /stats/top-vulnerabilities`
- **Auth**: ✅ Yes (cached)
- **Response**: `[ { "title": "...", "count": 5, "avg_severity": 4.2 } ]`

### 6. Risk Trends
- **Endpoint**: `GET /stats/trends`
- **Auth**: ✅ Yes (cached)
- **Response**: `[ { "date": "2026-03-10", "total": 45, "open": 12 } ]`

### 7. Dashboard Metrics
- **Endpoint**: `GET /dashboard/metrics`
- **Auth**: ✅ Yes
- **Response**: `{ "active_users": 5, "total_risks": 45, "critical_issues": 1 }`

### 8. Risk Trends (Dashboard)
- **Endpoint**: `GET /dashboard/risk-trends`
- **Auth**: ✅ Yes
- **Response**: Multi-period risk trend data

### 9. Severity Distribution
- **Endpoint**: `GET /dashboard/severity-distribution`
- **Auth**: ✅ Yes
- **Response**: `{ "LOW": 20, "MEDIUM": 15, "HIGH": 8, "CRITICAL": 2 }`

### 10. Mitigation Status
- **Endpoint**: `GET /dashboard/mitigation-status`
- **Auth**: ✅ Yes
- **Response**: Status breakdown with percentages

### 11. Top Risks
- **Endpoint**: `GET /dashboard/top-risks`
- **Auth**: ✅ Yes
- **Response**: `[ { "id": "uuid", "title": "...", "score": 85 } ]`

### 12. Mitigation Progress
- **Endpoint**: `GET /dashboard/mitigation-progress`
- **Auth**: ✅ Yes
- **Response**: Timeline and progress data

### 13. Complete Dashboard
- **Endpoint**: `GET /dashboard/complete`
- **Auth**: ✅ Yes
- **Response**: All dashboard data combined

### 14. Analytics - Risk Metrics
- **Endpoint**: `GET /analytics/risks/metrics`
- **Auth**: ✅ Yes
- **Response**: Detailed risk metrics

### 15. Analytics - Risk Trends
- **Endpoint**: `GET /analytics/risks/trends`
- **Auth**: ✅ Yes
- **Response**: Trend analysis

### 16. Analytics - Mitigation Metrics
- **Endpoint**: `GET /analytics/mitigations/metrics`
- **Auth**: ✅ Yes
- **Response**: Mitigation metrics

### 17. Analytics - Frameworks
- **Endpoint**: `GET /analytics/frameworks`
- **Auth**: ✅ Yes
- **Response**: Framework compliance metrics

### 18. Analytics - Dashboard Snapshot
- **Endpoint**: `GET /analytics/dashboard`
- **Auth**: ✅ Yes
- **Response**: Dashboard snapshot

### 19. Analytics - Export Data
- **Endpoint**: `GET /analytics/export`
- **Auth**: ✅ Yes
- **Response**: Exportable analytics data

---

## Export (1 endpoint)

### 1. Export Risks to PDF
- **Endpoint**: `GET /export/pdf`
- **Auth**: ✅ Yes
- **Query**: `format=pdf|excel`
- **Response**: Binary file (PDF or Excel)

---

## User Management (5 endpoints)

### 1. List Users
- **Endpoint**: `GET /users`
- **Auth**: ✅ Yes (admin+)
- **Response**: `[ { "id": "uuid", "email": "...", "role": "..." } ]`

### 2. Create User
- **Endpoint**: `POST /users`
- **Auth**: ✅ Yes (admin+)
- **Body**: `{ "email": "...", "first_name": "...", "last_name": "...", "role": "..." }`
- **Response**: `{ "id": "uuid", ... }`

### 3. Update User Status
- **Endpoint**: `PATCH /users/:id/status`
- **Auth**: ✅ Yes (admin+)
- **Body**: `{ "status": "active|inactive" }`
- **Response**: `{ "id": "uuid", "status": "..." }`

### 4. Update User Role
- **Endpoint**: `PATCH /users/:id/role`
- **Auth**: ✅ Yes (admin+)
- **Body**: `{ "role": "admin|analyst|viewer" }`
- **Response**: `{ "id": "uuid", "role": "..." }`

### 5. Delete User
- **Endpoint**: `DELETE /users/:id`
- **Auth**: ✅ Yes (admin+)
- **Response**: `204 No Content`

### 6. Update User Profile
- **Endpoint**: `PATCH /users/:id`
- **Auth**: ✅ Yes (own user or admin)
- **Body**: `{ "first_name": "...", "last_name": "...", "avatar_url": "..." }`
- **Response**: `{ "id": "uuid", ... }`

---

## Team Management (7 endpoints)

### 1. Create Team
- **Endpoint**: `POST /teams`
- **Auth**: ✅ Yes (admin+)
- **Body**: `{ "name": "Team name", "description": "..." }`
- **Response**: `{ "id": "uuid", ... }`

### 2. List Teams
- **Endpoint**: `GET /teams`
- **Auth**: ✅ Yes (admin+)
- **Response**: `[ { "id": "uuid", "name": "...", "member_count": 5 } ]`

### 3. Get Team
- **Endpoint**: `GET /teams/:id`
- **Auth**: ✅ Yes (admin+)
- **Response**: `{ "id": "uuid", "name": "...", "members": [...] }`

### 4. Update Team
- **Endpoint**: `PATCH /teams/:id`
- **Auth**: ✅ Yes (admin+)
- **Body**: `{ "name": "...", "description": "..." }`
- **Response**: `{ "id": "uuid", ... }`

### 5. Delete Team
- **Endpoint**: `DELETE /teams/:id`
- **Auth**: ✅ Yes (admin+)
- **Response**: `204 No Content`

### 6. Add Team Member
- **Endpoint**: `POST /teams/:id/members/:userId`
- **Auth**: ✅ Yes (admin+)
- **Response**: `{ "team_id": "uuid", "user_id": "uuid" }`

### 7. Remove Team Member
- **Endpoint**: `DELETE /teams/:id/members/:userId`
- **Auth**: ✅ Yes (admin+)
- **Response**: `204 No Content`

---

## RBAC Management (23 endpoints)

### User Management
- `GET /rbac/users` - List users
- `POST /rbac/users` - Add user to tenant
- `GET /rbac/users/:user_id` - Get user
- `PATCH /rbac/users/:user_id/role` - Change role
- `DELETE /rbac/users/:user_id` - Remove user
- `GET /rbac/users/:user_id/permissions` - Get permissions
- `GET /rbac/users/stats` - Tenant user stats

### Role Management
- `GET /rbac/roles` - List roles
- `POST /rbac/roles` - Create role
- `GET /rbac/roles/:role_id` - Get role
- `PATCH /rbac/roles/:role_id` - Update role
- `DELETE /rbac/roles/:role_id` - Delete role
- `GET /rbac/roles/:role_id/permissions` - Get permissions
- `POST /rbac/roles/:role_id/permissions` - Assign permission
- `DELETE /rbac/roles/:role_id/permissions` - Remove permission

### Tenant Management
- `GET /rbac/tenants` - List tenants
- `POST /rbac/tenants` - Create tenant
- `GET /rbac/tenants/:tenant_id` - Get tenant
- `PATCH /rbac/tenants/:tenant_id` - Update tenant
- `DELETE /rbac/tenants/:tenant_id` - Delete tenant
- `GET /rbac/tenants/:tenant_id/users` - List users
- `GET /rbac/tenants/:tenant_id/stats` - Get stats

---

## Audit Logs (3 endpoints)

### 1. List Audit Logs
- **Endpoint**: `GET /audit-logs`
- **Auth**: ✅ Yes (admin+)
- **Query**: `page, limit, action, user_id, date_from, date_to`
- **Response**: `[ { "id": "uuid", "action": "...", "user_id": "...", "timestamp": "..." } ]`

### 2. Get User Audit Logs
- **Endpoint**: `GET /audit-logs/user/:user_id`
- **Auth**: ✅ Yes (admin+)
- **Response**: User-specific audit logs

### 3. Get Logs by Action
- **Endpoint**: `GET /audit-logs/action/:action`
- **Auth**: ✅ Yes (admin+)
- **Response**: Logs filtered by action type

---

## API Tokens (7 endpoints)

### 1. Create Token
- **Endpoint**: `POST /tokens`
- **Auth**: ✅ Yes
- **Body**: `{ "name": "CI/CD", "description": "...", "expires_at": "2026-12-31T..." }`
- **Response**: `{ "id": "uuid", "token": "opnrsk_...", ... }`

### 2. List Tokens
- **Endpoint**: `GET /tokens`
- **Auth**: ✅ Yes
- **Response**: `[ { "id": "uuid", "name": "...", "created_at": "..." } ]`

### 3. Get Token
- **Endpoint**: `GET /tokens/:id`
- **Auth**: ✅ Yes
- **Response**: `{ "id": "uuid", ... }`

### 4. Update Token
- **Endpoint**: `PUT /tokens/:id`
- **Auth**: ✅ Yes
- **Body**: `{ "name": "...", "expires_at": "..." }`
- **Response**: `{ "id": "uuid", ... }`

### 5. Revoke Token
- **Endpoint**: `POST /tokens/:id/revoke`
- **Auth**: ✅ Yes
- **Response**: `204 No Content`

### 6. Rotate Token
- **Endpoint**: `POST /tokens/:id/rotate`
- **Auth**: ✅ Yes
- **Response**: `{ "token": "opnrsk_new_token..." }`

### 7. Delete Token
- **Endpoint**: `DELETE /tokens/:id`
- **Auth**: ✅ Yes
- **Response**: `204 No Content`

---

## Custom Fields (7 endpoints)

### 1. Create Custom Field
- **Endpoint**: `POST /custom-fields`
- **Auth**: ✅ Yes
- **Body**: `{ "name": "...", "type": "text|number|select", "scope": "risk|mitigation|asset" }`
- **Response**: `{ "id": "uuid", ... }`

### 2. List Custom Fields
- **Endpoint**: `GET /custom-fields`
- **Auth**: ✅ Yes
- **Response**: `[ { "id": "uuid", "name": "...", ... } ]`

### 3. Get Custom Field
- **Endpoint**: `GET /custom-fields/:id`
- **Auth**: ✅ Yes
- **Response**: `{ "id": "uuid", ... }`

### 4. Update Custom Field
- **Endpoint**: `PATCH /custom-fields/:id`
- **Auth**: ✅ Yes
- **Body**: `{ "name": "...", "required": true }`
- **Response**: `{ "id": "uuid", ... }`

### 5. Delete Custom Field
- **Endpoint**: `DELETE /custom-fields/:id`
- **Auth**: ✅ Yes
- **Response**: `204 No Content`

### 6. List by Scope
- **Endpoint**: `GET /custom-fields/scope/:scope`
- **Auth**: ✅ Yes
- **Response**: Custom fields for scope

### 7. Apply Template
- **Endpoint**: `POST /custom-fields/templates/:id/apply`
- **Auth**: ✅ Yes
- **Response**: Applied template results

---

## Bulk Operations (3 endpoints)

### 1. Create Bulk Operation
- **Endpoint**: `POST /bulk-operations`
- **Auth**: ✅ Yes
- **Body**: `{ "type": "update|delete", "filter": {...}, "updates": {...} }`
- **Response**: `{ "id": "uuid", "status": "queued", ... }`

### 2. List Bulk Operations
- **Endpoint**: `GET /bulk-operations`
- **Auth**: ✅ Yes
- **Response**: `[ { "id": "uuid", "status": "completed", ... } ]`

### 3. Get Bulk Operation
- **Endpoint**: `GET /bulk-operations/:id`
- **Auth**: ✅ Yes
- **Response**: `{ "id": "uuid", "status": "...", "progress": 75 }`

---

## Risk Timeline (6 endpoints)

### 1. Get Risk Timeline
- **Endpoint**: `GET /risks/:id/timeline`
- **Auth**: ✅ Yes
- **Response**: `[ { "timestamp": "...", "type": "status_change|score_change", "data": {...} } ]`

### 2. Get Status Changes
- **Endpoint**: `GET /risks/:id/timeline/status-changes`
- **Auth**: ✅ Yes
- **Response**: Status change events

### 3. Get Score Changes
- **Endpoint**: `GET /risks/:id/timeline/score-changes`
- **Auth**: ✅ Yes
- **Response**: Score change events

### 4. Get Timeline Trend
- **Endpoint**: `GET /risks/:id/timeline/trend`
- **Auth**: ✅ Yes
- **Response**: Trend data

### 5. Get Changes by Type
- **Endpoint**: `GET /risks/:id/timeline/changes/:type`
- **Auth**: ✅ Yes
- **Response**: Changes of specific type

### 6. Get Recent Activity
- **Endpoint**: `GET /timeline/recent`
- **Auth**: ✅ Yes
- **Response**: Recent activity across all risks

---

## Marketplace (14 endpoints)

### Browse
- `GET /marketplace/connectors` - List connectors
- `GET /marketplace/connectors/:id` - Get connector
- `GET /marketplace/connectors/search` - Search connectors

### Manage Apps
- `POST /marketplace/apps` - Install app
- `GET /marketplace/apps` - List apps
- `GET /marketplace/apps/:id` - Get app
- `PUT /marketplace/apps/:id` - Update app
- `POST /marketplace/apps/:id/enable` - Enable app
- `POST /marketplace/apps/:id/disable` - Disable app
- `DELETE /marketplace/apps/:id` - Uninstall app
- `PUT /marketplace/apps/:id/sync` - Update sync
- `POST /marketplace/apps/:id/sync` - Trigger sync
- `GET /marketplace/apps/:id/logs` - Get logs

### Reviews
- `POST /marketplace/connectors/:id/reviews` - Add review

---

## Gamification (1 endpoint)

### 1. Get User Profile
- **Endpoint**: `GET /gamification/me`
- **Auth**: ✅ Yes
- **Response**: `{ "level": 5, "points": 2850, "badges": [...], "rank": "Expert" }`

---

## Integrations (1 endpoint)

### 1. Test Integration
- **Endpoint**: `POST /integrations/:id/test`
- **Auth**: ✅ Yes
- **Body**: `{ "url": "...", "api_key": "..." }`
- **Response**: `{ "success": true, "message": "Connection successful" }`

---

## Summary

| Category | Count | Auth Required |
|----------|-------|---|
| Health & Status | 1 | ❌ |
| Authentication | 9 | ❌ |
| Risks | 5 | ✅ |
| Mitigations | 7 | ✅ |
| Assets | 2 | ✅ |
| Statistics | 19 | ✅ |
| Export | 1 | ✅ |
| Users | 6 | ✅ |
| Teams | 7 | ✅ |
| RBAC | 23 | ✅ |
| Audit | 3 | ✅ |
| Tokens | 7 | ✅ |
| Custom Fields | 7 | ✅ |
| Bulk Ops | 3 | ✅ |
| Timeline | 6 | ✅ |
| Marketplace | 14 | ✅ |
| Gamification | 1 | ✅ |
| Integrations | 1 | ✅ |
| **TOTAL** | **123** | |

---

**For detailed information, see:**
- [API_REFERENCE.md](./API_REFERENCE.md)
- [API_SECURITY_GUIDE.md](./API_SECURITY_GUIDE.md)
- [API_EXAMPLES.md](./API_EXAMPLES.md)
- [openapi.yaml](./openapi.yaml)
