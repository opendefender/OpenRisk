# API Examples & Code Samples

**Version**: 1.0  
**Last Updated**: March 10, 2026  
**Languages**: Bash (curl), Python, JavaScript  

---

## Table of Contents

1. [Authentication Examples](#authentication-examples)
2. [Risks CRUD](#risks-crud)
3. [Mitigations Management](#mitigations-management)
4. [Assets](#assets)
5. [Statistics & Dashboard](#statistics--dashboard)
6. [Advanced Features](#advanced-features)

---

## Authentication Examples

### Login with Email/Password

#### cURL

```bash
curl -X POST https://api.openrisk.io/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "secure_password"
  }' | jq .
```

#### Python

```python
import requests
import json

response = requests.post(
    'https://api.openrisk.io/api/v1/auth/login',
    json={
        'email': 'user@example.com',
        'password': 'secure_password'
    }
)

data = response.json()
access_token = data['access_token']
print(f"Token: {access_token}")
```

#### JavaScript (Node.js)

```javascript
const axios = require('axios');

const response = await axios.post('https://api.openrisk.io/api/v1/auth/login', {
  email: 'user@example.com',
  password: 'secure_password'
});

const accessToken = response.data.access_token;
console.log(`Token: ${accessToken}`);
```

#### JavaScript (Browser)

```javascript
const response = await fetch('https://api.openrisk.io/api/v1/auth/login', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    email: 'user@example.com',
    password: 'secure_password'
  })
});

const data = await response.json();
const accessToken = data.access_token;
console.log(`Token: ${accessToken}`);

// Store in localStorage for later use
localStorage.setItem('openrisk_token', accessToken);
```

### Refresh Token

#### cURL

```bash
curl -X POST https://api.openrisk.io/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }' | jq .
```

#### Python

```python
response = requests.post(
    'https://api.openrisk.io/api/v1/auth/refresh',
    json={'refresh_token': refresh_token}
)

new_token = response.json()['access_token']
```

### Get Current User

#### cURL

```bash
TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."

curl https://api.openrisk.io/api/v1/users/me \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### Python

```python
headers = {'Authorization': f'Bearer {access_token}'}

response = requests.get(
    'https://api.openrisk.io/api/v1/users/me',
    headers=headers
)

user = response.json()
print(f"User: {user['email']}, Role: {user['role']}")
```

#### JavaScript

```javascript
const response = await fetch('https://api.openrisk.io/api/v1/users/me', {
  headers: {
    'Authorization': `Bearer ${accessToken}`
  }
});

const user = await response.json();
console.log(`User: ${user.email}, Role: ${user.role}`);
```

---

## Risks CRUD

### List Risks

#### cURL

```bash
TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."

# Simple list
curl https://api.openrisk.io/api/v1/risks \
  -H "Authorization: Bearer $TOKEN" | jq .

# With pagination
curl 'https://api.openrisk.io/api/v1/risks?page=1&limit=10' \
  -H "Authorization: Bearer $TOKEN" | jq .

# With sorting
curl 'https://api.openrisk.io/api/v1/risks?sort_by=-created_at' \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### Python

```python
import requests

headers = {'Authorization': f'Bearer {access_token}'}

# Simple list
response = requests.get(
    'https://api.openrisk.io/api/v1/risks',
    headers=headers
)
risks = response.json()
print(f"Found {len(risks)} risks")

# With pagination
response = requests.get(
    'https://api.openrisk.io/api/v1/risks',
    headers=headers,
    params={'page': 1, 'limit': 10}
)

# With sorting
response = requests.get(
    'https://api.openrisk.io/api/v1/risks',
    headers=headers,
    params={'sort_by': '-created_at'}
)
```

#### JavaScript

```javascript
const token = localStorage.getItem('openrisk_token');

// Simple list
const response = await fetch('https://api.openrisk.io/api/v1/risks', {
  headers: { 'Authorization': `Bearer ${token}` }
});
const risks = await response.json();

// With pagination
const response = await fetch(
  'https://api.openrisk.io/api/v1/risks?page=1&limit=10',
  { headers: { 'Authorization': `Bearer ${token}` } }
);

// With sorting
const response = await fetch(
  'https://api.openrisk.io/api/v1/risks?sort_by=-created_at',
  { headers: { 'Authorization': `Bearer ${token}` } }
);
```

### Create Risk

#### cURL

```bash
curl -X POST https://api.openrisk.io/api/v1/risks \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Unpatched Server Vulnerability",
    "description": "Critical security patch not applied to production server",
    "impact": 5,
    "probability": 4,
    "tags": ["production", "infrastructure", "critical"],
    "asset_ids": ["550e8400-e29b-41d4-a716-446655440000"],
    "frameworks": ["ISO_27001", "CIS"]
  }' | jq .
```

#### Python

```python
risk_data = {
    "title": "Unpatched Server Vulnerability",
    "description": "Critical security patch not applied to production server",
    "impact": 5,
    "probability": 4,
    "tags": ["production", "infrastructure", "critical"],
    "asset_ids": ["550e8400-e29b-41d4-a716-446655440000"],
    "frameworks": ["ISO_27001", "CIS"]
}

response = requests.post(
    'https://api.openrisk.io/api/v1/risks',
    json=risk_data,
    headers={'Authorization': f'Bearer {access_token}'}
)

risk = response.json()
print(f"Created risk: {risk['id']}")
```

#### JavaScript

```javascript
const riskData = {
  title: "Unpatched Server Vulnerability",
  description: "Critical security patch not applied",
  impact: 5,
  probability: 4,
  tags: ["production", "infrastructure", "critical"],
  asset_ids: ["550e8400-e29b-41d4-a716-446655440000"],
  frameworks: ["ISO_27001", "CIS"]
};

const response = await fetch('https://api.openrisk.io/api/v1/risks', {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify(riskData)
});

const risk = await response.json();
console.log(`Created risk: ${risk.id}`);
```

### Get Specific Risk

#### cURL

```bash
RISK_ID="550e8400-e29b-41d4-a716-446655440001"

curl https://api.openrisk.io/api/v1/risks/$RISK_ID \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### Python

```python
risk_id = "550e8400-e29b-41d4-a716-446655440001"

response = requests.get(
    f'https://api.openrisk.io/api/v1/risks/{risk_id}',
    headers={'Authorization': f'Bearer {access_token}'}
)

risk = response.json()
print(f"Risk: {risk['title']}, Status: {risk['status']}")
```

### Update Risk

#### cURL

```bash
curl -X PATCH https://api.openrisk.io/api/v1/risks/$RISK_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Updated Title",
    "status": "open",
    "probability": 3
  }' | jq .
```

#### Python

```python
update_data = {
    "title": "Updated Title",
    "status": "open",
    "probability": 3
}

response = requests.patch(
    f'https://api.openrisk.io/api/v1/risks/{risk_id}',
    json=update_data,
    headers={'Authorization': f'Bearer {access_token}'}
)
```

### Delete Risk

#### cURL

```bash
curl -X DELETE https://api.openrisk.io/api/v1/risks/$RISK_ID \
  -H "Authorization: Bearer $TOKEN"
```

#### Python

```python
response = requests.delete(
    f'https://api.openrisk.io/api/v1/risks/{risk_id}',
    headers={'Authorization': f'Bearer {access_token}'}
)

if response.status_code == 204:
    print("Risk deleted successfully")
```

---

## Mitigations Management

### Add Mitigation to Risk

#### cURL

```bash
RISK_ID="550e8400-e29b-41d4-a716-446655440001"

curl -X POST https://api.openrisk.io/api/v1/risks/$RISK_ID/mitigations \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Apply security patch",
    "assignee": "alice@example.com",
    "due_date": "2026-03-31T23:59:59Z",
    "cost": 2,
    "mitigation_time": 4
  }' | jq .
```

#### Python

```python
mitigation_data = {
    "title": "Apply security patch",
    "assignee": "alice@example.com",
    "due_date": "2026-03-31T23:59:59Z",
    "cost": 2,
    "mitigation_time": 4
}

response = requests.post(
    f'https://api.openrisk.io/api/v1/risks/{risk_id}/mitigations',
    json=mitigation_data,
    headers={'Authorization': f'Bearer {access_token}'}
)

mitigation = response.json()
print(f"Created mitigation: {mitigation['id']}")
```

### Update Mitigation

#### cURL

```bash
MITIGATION_ID="660f9511-f40d-52e5-b827-557766551002"

curl -X PATCH https://api.openrisk.io/api/v1/mitigations/$MITIGATION_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Updated mitigation title",
    "progress": 75,
    "status": "IN_PROGRESS"
  }' | jq .
```

#### Python

```python
update_data = {
    "title": "Updated mitigation title",
    "progress": 75,
    "status": "IN_PROGRESS"
}

response = requests.patch(
    f'https://api.openrisk.io/api/v1/mitigations/{mitigation_id}',
    json=update_data,
    headers={'Authorization': f'Bearer {access_token}'}
)
```

### Toggle Mitigation Status

#### cURL

```bash
curl -X PATCH https://api.openrisk.io/api/v1/mitigations/$MITIGATION_ID/toggle \
  -H "Authorization: Bearer $TOKEN"
```

#### Python

```python
response = requests.patch(
    f'https://api.openrisk.io/api/v1/mitigations/{mitigation_id}/toggle',
    headers={'Authorization': f'Bearer {access_token}'}
)
```

### Add Sub-Action

#### cURL

```bash
curl -X POST https://api.openrisk.io/api/v1/mitigations/$MITIGATION_ID/subactions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Download and test patch"
  }' | jq .
```

#### Python

```python
response = requests.post(
    f'https://api.openrisk.io/api/v1/mitigations/{mitigation_id}/subactions',
    json={"title": "Download and test patch"},
    headers={'Authorization': f'Bearer {access_token}'}
)

subaction = response.json()
print(f"Created sub-action: {subaction['id']}")
```

### Toggle Sub-Action Completion

#### cURL

```bash
SUBACTION_ID="770g0622-g51e-63f6-c938-668877662003"

curl -X PATCH https://api.openrisk.io/api/v1/mitigations/$MITIGATION_ID/subactions/$SUBACTION_ID/toggle \
  -H "Authorization: Bearer $TOKEN"
```

---

## Assets

### List Assets

#### cURL

```bash
curl https://api.openrisk.io/api/v1/assets \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### Python

```python
response = requests.get(
    'https://api.openrisk.io/api/v1/assets',
    headers={'Authorization': f'Bearer {access_token}'}
)

assets = response.json()
for asset in assets:
    print(f"Asset: {asset['name']} ({asset['type']})")
```

### Create Asset

#### cURL

```bash
curl -X POST https://api.openrisk.io/api/v1/assets \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Production Database",
    "type": "database",
    "description": "Primary PostgreSQL database",
    "location": "AWS us-east-1",
    "owner": "database-team@example.com"
  }' | jq .
```

#### Python

```python
asset_data = {
    "name": "Production Database",
    "type": "database",
    "description": "Primary PostgreSQL database",
    "location": "AWS us-east-1",
    "owner": "database-team@example.com"
}

response = requests.post(
    'https://api.openrisk.io/api/v1/assets',
    json=asset_data,
    headers={'Authorization': f'Bearer {access_token}'}
)

asset = response.json()
print(f"Created asset: {asset['id']}")
```

---

## Statistics & Dashboard

### Get Dashboard Stats

#### cURL

```bash
curl https://api.openrisk.io/api/v1/stats \
  -H "Authorization: Bearer $TOKEN" | jq .
```

**Response**:
```json
{
  "total_risks": 45,
  "open_risks": 12,
  "high_severity_risks": 3,
  "critical_severity_risks": 1,
  "mitigated_risks": 20,
  "pending_mitigations": 8,
  "average_risk_score": 3.2
}
```

### Get Risk Matrix

#### cURL

```bash
curl https://api.openrisk.io/api/v1/stats/risk-matrix \
  -H "Authorization: Bearer $TOKEN" | jq .
```

**Response**:
```json
{
  "matrix": [
    [0, 2, 5, 3, 1],
    [1, 4, 8, 6, 2],
    [2, 6, 12, 9, 4],
    [1, 5, 10, 8, 3],
    [0, 2, 4, 3, 1]
  ],
  "total_risks": 45
}
```

### Get Risk Trends

#### cURL

```bash
curl https://api.openrisk.io/api/v1/stats/trends \
  -H "Authorization: Bearer $TOKEN" | jq .
```

**Response**:
```json
{
  "trends": [
    { "date": "2026-02-10", "total": 40, "open": 10, "closed": 30 },
    { "date": "2026-02-17", "total": 42, "open": 11, "closed": 31 },
    { "date": "2026-02-24", "total": 44, "open": 12, "closed": 32 },
    { "date": "2026-03-03", "total": 45, "open": 12, "closed": 33 },
    { "date": "2026-03-10", "total": 45, "open": 12, "closed": 33 }
  ]
}
```

### Get Complete Dashboard

#### cURL

```bash
curl https://api.openrisk.io/api/v1/dashboard/complete \
  -H "Authorization: Bearer $TOKEN" | jq .
```

---

## Advanced Features

### Export Risks to PDF

#### cURL

```bash
curl https://api.openrisk.io/api/v1/export/pdf \
  -H "Authorization: Bearer $TOKEN" \
  -o risks_report.pdf
```

#### Python

```python
response = requests.get(
    'https://api.openrisk.io/api/v1/export/pdf',
    headers={'Authorization': f'Bearer {access_token}'}
)

with open('risks_report.pdf', 'wb') as f:
    f.write(response.content)
```

### Get Recommended Mitigations

#### cURL

```bash
curl https://api.openrisk.io/api/v1/mitigations/recommended \
  -H "Authorization: Bearer $TOKEN" | jq .
```

**Response**:
```json
[
  {
    "risk_id": "550e8400-e29b-41d4-a716-446655440001",
    "risk_title": "Unpatched Server",
    "mitigation": "Apply latest security patch",
    "effort_days": 2,
    "cost": 2,
    "spp_score": 0.95
  },
  {
    "risk_id": "550e8400-e29b-41d4-a716-446655440002",
    "risk_title": "Weak Passwords",
    "mitigation": "Enforce strong password policy",
    "effort_days": 5,
    "cost": 2,
    "spp_score": 0.87
  }
]
```

### Get User Gamification Profile

#### cURL

```bash
curl https://api.openrisk.io/api/v1/gamification/me \
  -H "Authorization: Bearer $TOKEN" | jq .
```

**Response**:
```json
{
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "level": 5,
  "points": 2850,
  "badges": ["First Risk", "Risk Analyst", "Mitigation Master"],
  "streak": 12,
  "rank": "Expert"
}
```

---

## Error Handling

### Handle Common Errors

#### Python

```python
import requests

def make_api_call(method, endpoint, data=None):
    url = f'https://api.openrisk.io/api/v1{endpoint}'
    headers = {'Authorization': f'Bearer {access_token}'}
    
    try:
        if method == 'GET':
            response = requests.get(url, headers=headers)
        elif method == 'POST':
            response = requests.post(url, json=data, headers=headers)
        elif method == 'PATCH':
            response = requests.patch(url, json=data, headers=headers)
        
        # Check for errors
        if response.status_code == 401:
            print("Error: Unauthorized - token may be expired")
            # Refresh token
            refresh_token()
            return make_api_call(method, endpoint, data)
        
        elif response.status_code == 403:
            print("Error: Forbidden - insufficient permissions")
            return None
        
        elif response.status_code == 429:
            print("Error: Rate limited - wait before retrying")
            return None
        
        elif response.status_code == 400:
            error = response.json()
            print(f"Error: {error.get('error')}")
            print(f"Details: {error.get('details')}")
            return None
        
        elif response.status_code >= 500:
            print("Error: Server error - try again later")
            return None
        
        return response.json()
    
    except requests.exceptions.RequestException as e:
        print(f"Network error: {e}")
        return None
```

#### JavaScript

```javascript
async function makeApiCall(method, endpoint, data = null) {
  const url = `https://api.openrisk.io/api/v1${endpoint}`;
  const headers = {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json'
  };

  try {
    const response = await fetch(url, {
      method,
      headers,
      body: data ? JSON.stringify(data) : null
    });

    if (response.status === 401) {
      console.error('Unauthorized - token expired');
      await refreshToken();
      return makeApiCall(method, endpoint, data);
    } else if (response.status === 403) {
      console.error('Forbidden - insufficient permissions');
      return null;
    } else if (response.status === 429) {
      console.error('Rate limited - wait before retrying');
      return null;
    } else if (response.status === 400) {
      const error = await response.json();
      console.error(`Validation error: ${error.error}`);
      console.error(`Details: ${error.details}`);
      return null;
    } else if (response.status >= 500) {
      console.error('Server error - try again later');
      return null;
    }

    return response.json();
  } catch (error) {
    console.error(`Network error: ${error.message}`);
    return null;
  }
}
```

---

## Best Practices

1. **Always validate responses** - Check status codes and error messages
2. **Implement retry logic** - Use exponential backoff for failed requests
3. **Cache tokens** - Store tokens securely for reuse
4. **Set request timeouts** - Prevent hanging requests
5. **Log API calls** - For debugging and audit purposes
6. **Handle rate limits** - Implement backoff and queue logic
7. **Sanitize inputs** - Validate all user inputs before sending
8. **Use pagination** - For large datasets, use page/limit parameters

---

**Questions?** See [API_REFERENCE.md](./API_REFERENCE.md) or [API_SECURITY_GUIDE.md](./API_SECURITY_GUIDE.md)
