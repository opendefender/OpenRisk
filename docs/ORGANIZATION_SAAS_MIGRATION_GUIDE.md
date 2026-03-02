# Organization Management & SaaS Migration Guide

**Date**: March 2, 2026  
**Phase**: 6B - SaaS & Organization Features  
**Status**: Complete  
**Version**: 1.0

---

## Executive Summary

This guide describes OpenRisk's new organization management system and data migration features, enabling both multi-user organizations and seamless migration from self-hosted to SaaS deployments.

---

## Part 1: Organization Management System

### Feature Overview

The organization management system provides:

1. **Multi-Organization Support**: Users can manage multiple organizations
2. **Team Management**: Create teams within organizations
3. **Role-Based Access Control**: Granular permission management
4. **Subscription Tiers**: Freemium, Professional, and Enterprise plans
5. **Feature Flags**: Different features per subscription tier
6. **Usage Tracking**: Monitor API calls, user count, and storage

### Subscription Tiers

#### Freemium (Free)
- **Price**: $0/month
- **Users**: 1
- **Risks**: 10
- **Features**:
  - Basic risk management
  - Audit logs
  - Community support
  - 100 API calls/month
- **API Access**: Limited

#### Professional ($29.99/month)
- **Users**: 10
- **Risks**: 1,000
- **Features**:
  - Advanced analytics
  - Custom reports
  - Full API access
  - Data export
  - Advanced compliance
  - Custom fields
  - Webhooks
  - 100,000 API calls/month
- **Support**: Email support

#### Enterprise ($99.99/month)
- **Users**: 1,000+
- **Risks**: 100,000+
- **Features**:
  - All Professional features
  - Single Sign-On (SSO)
  - Dedicated support
  - Custom integrations
  - 10,000,000 API calls/month
- **Support**: Dedicated support team

### Organization Roles

| Role | Permissions |
|------|-------------|
| **Owner** | Full access, billing, member management, organization settings |
| **Admin** | All except billing and organization deletion |
| **Manager** | Manage risks, mitigations, assets, team management |
| **Member** | Create and view own resources, collaborate on shared items |
| **Viewer** | Read-only access |

---

## Part 2: Organization API Reference

### Create Organization

```bash
POST /api/v1/organizations
Content-Type: application/json

{
  "name": "Acme Corp",
  "slug": "acme-corp",
  "description": "Acme Corporation",
  "website": "https://acme.example.com",
  "country": "US",
  "industry": "Technology",
  "company_size": "100-500",
  "timezone": "America/New_York"
}
```

**Response (201)**:
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Acme Corp",
  "slug": "acme-corp",
  "subscription_tier": "freemium",
  "subscription_status": "trial",
  "subscription_start_date": "2026-03-02T00:00:00Z",
  "features": {
    "max_users": 1,
    "max_risks": 10,
    "advanced_analytics": false,
    "api_access": false
  },
  "created_at": "2026-03-02T12:00:00Z"
}
```

### Get Organization

```bash
GET /api/v1/organizations/{organizationId}
Authorization: Bearer {token}
```

### Update Organization

```bash
PUT /api/v1/organizations/{organizationId}
Authorization: Bearer {token}
Content-Type: application/json

{
  "name": "Acme Corp Updated",
  "timezone": "America/Chicago"
}
```

### Upgrade Subscription

```bash
POST /api/v1/organizations/{organizationId}/upgrade
Authorization: Bearer {token}
Content-Type: application/json

{
  "tier": "pro"
}
```

**Response**:
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "subscription_tier": "pro",
  "features": {
    "max_users": 10,
    "max_risks": 1000,
    "advanced_analytics": true,
    "api_access": true,
    "custom_reports": true
  }
}
```

### Add Member to Organization

```bash
POST /api/v1/organizations/{organizationId}/members
Authorization: Bearer {token}
Content-Type: application/json

{
  "user_id": "660e8400-e29b-41d4-a716-446655440000",
  "role": "manager"
}
```

**Response (201)**:
```json
{
  "id": "770e8400-e29b-41d4-a716-446655440000",
  "organization_id": "550e8400-e29b-41d4-a716-446655440000",
  "user_id": "660e8400-e29b-41d4-a716-446655440000",
  "role": "manager",
  "status": "active",
  "joined_at": "2026-03-02T12:30:00Z"
}
```

### Get Organization Members

```bash
GET /api/v1/organizations/{organizationId}/members
Authorization: Bearer {token}
```

**Response**:
```json
[
  {
    "id": "770e8400-e29b-41d4-a716-446655440000",
    "organization_id": "550e8400-e29b-41d4-a716-446655440000",
    "user_id": "660e8400-e29b-41d4-a716-446655440000",
    "role": "manager",
    "user": {
      "id": "660e8400-e29b-41d4-a716-446655440000",
      "email": "manager@acme.example.com",
      "first_name": "John",
      "last_name": "Doe"
    }
  }
]
```

### Update Member Role

```bash
PATCH /api/v1/organizations/{organizationId}/members/{userId}/role
Authorization: Bearer {token}
Content-Type: application/json

{
  "role": "admin"
}
```

### Remove Member from Organization

```bash
DELETE /api/v1/organizations/{organizationId}/members/{userId}
Authorization: Bearer {token}
```

---

## Part 3: Data Migration System

### Migration Overview

The migration system enables users to:

1. **Export Data** from self-hosted OpenRisk instances
2. **Upload Data** to SaaS organization
3. **Validate** migrated data
4. **Link Accounts** between self-hosted and SaaS

### Supported Deployment Types

- `docker`: Docker-based self-hosted deployment
- `kubernetes`: Kubernetes-based self-hosted deployment
- `self-hosted`: Generic self-hosted deployment

### Migration Types

- **Full**: Migrate all data (risks, assets, mitigations, users, etc.)
- **Partial**: Migrate selected items
- **Backup**: Create backup without migration

### Data Export from Self-Hosted

```bash
# 1. Export data as JSON
docker exec openrisk-backend ./export_data.sh \
  --format json \
  --output /tmp/openrisk-data.json

# 2. Export PostgreSQL backup
pg_dump -h localhost -U openrisk -d openrisk_local \
  -F c -f /tmp/openrisk-backup.dump

# 3. Verify export
ls -lh /tmp/openrisk-*
```

### Data Export Script Example

```bash
#!/bin/bash
# export_data.sh

FORMAT=${1:-json}
OUTPUT=${2:-./openrisk-export.json}

# Connect to local database
psql $DATABASE_URL << EOF > "$OUTPUT"
-- Export risks
SELECT jsonb_build_object(
  'risks', (SELECT jsonb_agg(row_to_json(t)) FROM risks t),
  'assets', (SELECT jsonb_agg(row_to_json(t)) FROM assets t),
  'mitigations', (SELECT jsonb_agg(row_to_json(t)) FROM mitigations t),
  'custom_fields', (SELECT jsonb_agg(row_to_json(t)) FROM custom_fields t),
  'export_date', NOW(),
  'export_version', '1.0'
) AS data;
EOF

echo "Export completed: $OUTPUT"
ls -lh "$OUTPUT"
```

---

## Part 4: Migration API Reference

### Create Migration Job

```bash
POST /api/v1/migrations
Authorization: Bearer {token}
Content-Type: application/json

{
  "source_deployment_type": "docker",
  "source_database_version": "16.1",
  "source_data_size_bytes": 104857600,
  "target_organization_id": "550e8400-e29b-41d4-a716-446655440000",
  "target_user_id": "660e8400-e29b-41d4-a716-446655440000",
  "migration_type": "full"
}
```

**Response (201)**:
```json
{
  "id": "880e8400-e29b-41d4-a716-446655440000",
  "source_deployment_type": "docker",
  "target_organization_id": "550e8400-e29b-41d4-a716-446655440000",
  "migration_type": "full",
  "status": "pending",
  "created_at": "2026-03-02T13:00:00Z"
}
```

### Upload Migration Data

```bash
POST /api/v1/migrations/{jobId}/upload
Authorization: Bearer {token}
Content-Type: multipart/form-data

# Using curl
curl -X POST \
  -H "Authorization: Bearer {token}" \
  -F "file=@openrisk-data.json" \
  -F "fileType=json" \
  https://api.saas.example.com/api/v1/migrations/{jobId}/upload
```

**Response**:
```json
{
  "message": "File uploaded and processing started",
  "job_id": "880e8400-e29b-41d4-a716-446655440000",
  "status": "in_progress"
}
```

### Start Migration

```bash
POST /api/v1/migrations/{jobId}/start
Authorization: Bearer {token}
```

**Response**:
```json
{
  "id": "880e8400-e29b-41d4-a716-446655440000",
  "status": "in_progress",
  "started_at": "2026-03-02T13:05:00Z"
}
```

### Get Migration Status

```bash
GET /api/v1/migrations/{jobId}/status
Authorization: Bearer {token}
```

**Response**:
```json
{
  "job_id": "880e8400-e29b-41d4-a716-446655440000",
  "status": "in_progress",
  "migration_type": "full",
  "created_at": "2026-03-02T13:00:00Z",
  "started_at": "2026-03-02T13:05:00Z",
  "completed_at": null,
  "total_items": 1500,
  "migrated_items": 1200,
  "failed_items": 0,
  "skipped_items": 300,
  "items_by_status": {
    "migrated": 1200,
    "pending": 300,
    "failed": 0
  },
  "progress_percentage": 80.0
}
```

### Complete Migration

```bash
POST /api/v1/migrations/{jobId}/complete
Authorization: Bearer {token}
```

**Note**: Migration must have >95% success rate to complete.

**Response**:
```json
{
  "id": "880e8400-e29b-41d4-a716-446655440000",
  "status": "completed",
  "completed_at": "2026-03-02T14:00:00Z",
  "migrated_items": 1500,
  "validation_results": {
    "total_items": 1500,
    "migrated_items": 1500,
    "success_rate": 1.0,
    "validation_time": "2026-03-02T14:00:00Z"
  }
}
```

---

## Part 5: Step-by-Step Migration Guide

### For Self-Hosted Users Migrating to SaaS

#### Step 1: Prepare for Migration

```bash
# 1. Stop any write operations to the database
# OR take a database snapshot

# 2. Verify database integrity
psql -h localhost -U openrisk -d openrisk_local -c \
  "SELECT COUNT(*) FROM risks;"

# 3. Note down your database version
psql --version
```

#### Step 2: Export Data

```bash
# Option A: Export as JSON
./export_data.sh json openrisk-export.json

# Option B: Create database backup
pg_dump -h localhost -U openrisk -d openrisk_local \
  -F c -f openrisk-backup.dump

# Verify export file
ls -lh openrisk-export.json
file openrisk-export.json
```

#### Step 3: Create SaaS Account & Organization

```bash
# 1. Sign up on SaaS platform
# https://saas.openrisk.io/signup

# 2. Create organization
# POST /api/v1/organizations
# {
#   "name": "My Company",
#   "slug": "my-company",
#   "country": "US"
# }

ORG_ID="550e8400-e29b-41d4-a716-446655440000"
```

#### Step 4: Create Migration Job

```bash
USER_ID="660e8400-e29b-41d4-a716-446655440000"

curl -X POST https://api.saas.example.com/api/v1/migrations \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "source_deployment_type": "docker",
    "source_database_version": "16.1",
    "source_data_size_bytes": 104857600,
    "target_organization_id": "'$ORG_ID'",
    "target_user_id": "'$USER_ID'",
    "migration_type": "full"
  }' | jq '.id' > job_id.txt

JOB_ID=$(cat job_id.txt)
```

#### Step 5: Upload Data

```bash
curl -X POST https://api.saas.example.com/api/v1/migrations/$JOB_ID/upload \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@openrisk-export.json" \
  -F "fileType=json"
```

#### Step 6: Monitor Migration Progress

```bash
# Check status every 30 seconds
while true; do
  curl -s https://api.saas.example.com/api/v1/migrations/$JOB_ID/status \
    -H "Authorization: Bearer $TOKEN" | jq '.progress_percentage'
  
  sleep 30
done
```

#### Step 7: Validate Migration

```bash
# Once migration reaches 100%
curl -X POST https://api.saas.example.com/api/v1/migrations/$JOB_ID/complete \
  -H "Authorization: Bearer $TOKEN" | jq '.'
```

---

## Part 6: Account Linking

### Link Self-Hosted & SaaS Accounts

After migration, link your accounts to ensure future sync:

```bash
POST /api/v1/auth/link-accounts
Authorization: Bearer {saas_token}
Content-Type: application/json

{
  "self_hosted_url": "https://openrisk.mycompany.com",
  "self_hosted_email": "user@mycompany.com",
  "self_hosted_api_key": "xxx-yyy-zzz",
  "sync_preferences": {
    "auto_sync": true,
    "sync_frequency": "daily"
  }
}
```

---

## Part 7: Feature Comparison

| Feature | Freemium | Professional | Enterprise |
|---------|----------|--------------|-----------|
| Risk Management | ✓ | ✓ | ✓ |
| Basic Analytics | ✓ | ✓ | ✓ |
| Advanced Analytics | ✗ | ✓ | ✓ |
| Custom Reports | ✗ | ✓ | ✓ |
| API Access | ✗ | ✓ | ✓ |
| Webhooks | ✗ | ✓ | ✓ |
| SSO | ✗ | ✗ | ✓ |
| Dedicated Support | ✗ | ✗ | ✓ |
| Custom Fields | ✗ | ✓ | ✓ |
| Advanced Compliance | ✗ | ✓ | ✓ |
| Data Export | ✗ | ✓ | ✓ |
| Max Users | 1 | 10 | 1000+ |
| Max Risks | 10 | 1000 | 100000+ |
| API Calls/Month | 100 | 100K | 10M |

---

## Part 8: Troubleshooting

### Common Issues

#### Migration Stuck in Progress

```bash
# Check logs
curl -s https://api.saas.example.com/api/v1/migrations/$JOB_ID \
  -H "Authorization: Bearer $TOKEN" | jq '.migration_log'

# If stuck > 1 hour, cancel and retry
curl -X POST https://api.saas.example.com/api/v1/migrations/$JOB_ID/cancel \
  -H "Authorization: Bearer $TOKEN"
```

#### Validation Failure

```bash
# Check failed items
curl -s https://api.saas.example.com/api/v1/migrations/$JOB_ID \
  -H "Authorization: Bearer $TOKEN" | jq '.error_details'

# Retry migration with selective import
```

#### Data Mismatch

```bash
# Verify source data integrity
psql -d openrisk_local -c "SELECT COUNT(*) FROM risks;"
psql -d openrisk_local -c "SELECT SUM(octet_length(risk_data::text)) FROM risks;"

# Compare with migrated data
curl -s https://api.saas.example.com/api/v1/organizations/$ORG_ID \
  -H "Authorization: Bearer $TOKEN" | jq '.current_risk_count'
```

---

## Part 9: Best Practices

### Before Migration

1. **Test locally** with a copy of your database
2. **Backup everything** - create PostgreSQL dumps
3. **Document custom configurations** - field mappings, custom rules
4. **Notify users** - plan migration during low-traffic period
5. **Verify data** - check data integrity before export

### During Migration

1. **Monitor progress** - check status every 15 minutes
2. **Keep audit logs** - maintain records of what was migrated
3. **Test incrementally** - consider partial migration first
4. **Have fallback plan** - keep self-hosted instance running until verified

### After Migration

1. **Validate all data** - spot check critical records
2. **Test integrations** - verify API connections work
3. **Update configurations** - point applications to new SaaS instance
4. **Archive self-hosted data** - keep backup for compliance
5. **Train users** - explain new organization/team structure

---

## Support

For migration assistance, contact:

- **Email**: migration@openrisk.io
- **Documentation**: https://docs.openrisk.io/migration
- **Support Portal**: https://support.openrisk.io
- **Status Page**: https://status.openrisk.io

---

**Last Updated**: March 2, 2026
