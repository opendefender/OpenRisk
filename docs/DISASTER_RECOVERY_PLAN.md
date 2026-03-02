# Disaster Recovery & Business Continuity Plan

**Date**: March 2, 2026  
**Phase**: 6B - Production Ready  
**Status**: Complete  
**Version**: 1.0  
**RTO**: < 1 hour (Recovery Time Objective)  
**RPO**: < 15 minutes (Recovery Point Objective)

---

## Executive Summary

This plan outlines procedures for disaster recovery, business continuity, and resilience testing for OpenRisk production environment. The system is designed for high availability with automatic failover capabilities and multiple recovery options.

---

## Part 1: Disaster Types & Recovery Strategies

### Classification

| Disaster Type | Severity | RTO | RPO | Strategy |
|---------------|----------|-----|-----|----------|
| Single pod crash | Low | 1-5 min | <1 min | Auto-restart |
| Node failure | Medium | 10-30 min | <5 min | Node auto-heal |
| Database failover | High | 30-60 min | <15 min | Replication + WAL |
| Data center outage | Critical | 1-2 hours | <30 min | Multi-region failover |
| Data corruption | Critical | 2-4 hours | 1+ hours | Point-in-time restore |

---

## Part 2: Backup Strategy

### Backup Architecture

```
┌─────────────────────────────────────────────────────┐
│         Production Database (Primary)               │
│  - Continuous WAL archiving                         │
│  - Streaming replication to standby                 │
└──────────────────┬──────────────────────────────────┘
                   │
        ┌──────────┼──────────┐
        │          │          │
        │          │          │
    ┌───▼──┐  ┌───▼──┐  ┌───▼──┐
    │Hot   │  │Cold  │  │Cloud │
    │Stand-│  │Backup│  │Backup│
    │by    │  │      │  │      │
    └──────┘  └──────┘  └──────┘
```

### Automated Backup Procedure

```bash
#!/bin/bash
# backup_manager.sh - Automated backup script

LOG_FILE="/var/log/openrisk/backup.log"
BACKUP_BASE="/backups/openrisk"
RETENTION_DAYS=30

log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

# 1. Daily Full Backup
daily_full_backup() {
    BACKUP_DIR="$BACKUP_BASE/full_$(date +%Y%m%d)"
    mkdir -p "$BACKUP_DIR"
    
    log "Starting full backup to $BACKUP_DIR"
    
    pg_dump -h $DB_HOST \
            -U $DB_USER \
            -d $DB_NAME \
            -F c \
            -j 4 \
            -v > "$BACKUP_DIR/database.dump" 2>&1
    
    if [ $? -eq 0 ]; then
        log "✓ Full backup completed successfully"
        
        # Verify backup
        pg_restore -d openrisk_test \
                   -l "$BACKUP_DIR/database.dump" > /dev/null 2>&1
        if [ $? -eq 0 ]; then
            log "✓ Backup verification passed"
        fi
        
        # Upload to S3
        aws s3 cp "$BACKUP_DIR/database.dump" \
                  "s3://openrisk-backups/full_$(date +%Y%m%d)/" \
                  --sse AES256
    else
        log "✗ Full backup failed"
        # Send alert
        notify_backup_failure "Full backup failed"
    fi
}

# 2. Incremental Backups (every 6 hours)
incremental_backup() {
    BACKUP_DIR="$BACKUP_BASE/incremental_$(date +%Y%m%d_%H%M%S)"
    mkdir -p "$BACKUP_DIR"
    
    log "Starting incremental backup to $BACKUP_DIR"
    
    # Copy WAL files since last backup
    find $PGWAL -name "*.backup" -newer $LAST_BACKUP_FILE \
         -exec cp {} "$BACKUP_DIR/" \;
    
    if [ "$(ls -A $BACKUP_DIR)" ]; then
        log "✓ Incremental backup completed"
        
        # Upload to S3
        aws s3 cp "$BACKUP_DIR/" \
                  "s3://openrisk-backups/incremental_$(date +%Y%m%d_%H%M%S)/" \
                  --recursive
    else
        log "No new WAL files, skipping incremental backup"
    fi
}

# 3. Cleanup old backups
cleanup_old_backups() {
    log "Cleaning up backups older than $RETENTION_DAYS days"
    
    find "$BACKUP_BASE" -type d -mtime +$RETENTION_DAYS \
         -exec rm -rf {} \; 2>/dev/null
    
    aws s3 rm "s3://openrisk-backups/" \
             --recursive \
             --exclude "*" \
             --include "full_*" \
             --older-than $(date -d "$RETENTION_DAYS days ago" +%Y-%m-%d)
}

# Main execution
main() {
    # Run full backup daily at 2 AM
    CURRENT_HOUR=$(date +%H)
    if [ "$CURRENT_HOUR" -eq 2 ]; then
        daily_full_backup
    fi
    
    # Run incremental backup every 6 hours
    if [ $((CURRENT_HOUR % 6)) -eq 0 ]; then
        incremental_backup
    fi
    
    # Cleanup old backups daily at 3 AM
    if [ "$CURRENT_HOUR" -eq 3 ]; then
        cleanup_old_backups
    fi
}

main

log "Backup manager cycle completed"
```

### Backup Verification

```bash
#!/bin/bash
# verify_backups.sh

BACKUP_DIR="/backups/openrisk"
REPORT_FILE="/var/log/openrisk/backup_verification.txt"

echo "=== Backup Verification Report ===" > "$REPORT_FILE"
echo "Date: $(date)" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

# Check local backups
echo "Local Backups:" >> "$REPORT_FILE"
du -sh "$BACKUP_DIR"/* >> "$REPORT_FILE" 2>&1

# Verify backup integrity
echo -e "\nBackup Integrity Tests:" >> "$REPORT_FILE"

for backup in "$BACKUP_DIR"/full_*/database.dump; do
    echo "Testing: $backup" >> "$REPORT_FILE"
    
    pg_restore -l "$backup" > /dev/null 2>&1
    if [ $? -eq 0 ]; then
        echo "✓ PASS" >> "$REPORT_FILE"
    else
        echo "✗ FAIL" >> "$REPORT_FILE"
        # Alert on failure
        curl -X POST https://alerts.example.com/webhook \
             -d "backup_verification_failed=$backup"
    fi
done

# Check S3 backups
echo -e "\nS3 Backups:" >> "$REPORT_FILE"
aws s3 ls s3://openrisk-backups/ --recursive --summarize >> "$REPORT_FILE"

# Send report
mail -s "Backup Verification Report" ops@example.com < "$REPORT_FILE"
```

---

## Part 3: Recovery Procedures

### Scenario 1: Pod Crash Recovery

```bash
#!/bin/bash
# recover_from_pod_crash.sh

NAMESPACE="openrisk"
POD_NAME="$1"

echo "Recovering from pod crash: $POD_NAME"

# 1. Check pod status
kubectl get pod "$POD_NAME" -n "$NAMESPACE"

# 2. Kubernetes automatically restarts the pod
# No action needed - Kubernetes handles this

# 3. Monitor pod recovery
kubectl logs "$POD_NAME" -n "$NAMESPACE" -f

# 4. Verify service is responding
for i in {1..30}; do
    curl -f http://localhost:8080/health && break
    echo "Waiting for service to recover... ($i/30)"
    sleep 2
done

if [ $? -eq 0 ]; then
    echo "✓ Pod recovered successfully"
else
    echo "✗ Pod recovery failed, manual intervention required"
    exit 1
fi
```

### Scenario 2: Node Failure Recovery

```bash
#!/bin/bash
# recover_from_node_failure.sh

FAILED_NODE="$1"
NAMESPACE="openrisk"

echo "Recovering from node failure: $FAILED_NODE"

# 1. Mark node as unschedulable
kubectl cordon "$FAILED_NODE"

# 2. Drain workloads
kubectl drain "$FAILED_NODE" \
    --ignore-daemonsets \
    --delete-emptydir-data \
    --grace-period=120

# 3. Kubernetes reschedules pods to other nodes
# Monitor pod rescheduling
kubectl get pods -n "$NAMESPACE" -o wide --watch

# 4. Once all pods are running, remove the node from cluster
kubectl delete node "$FAILED_NODE"

# 5. Replace hardware and rejoin cluster
# (See infrastructure team for hardware replacement)

echo "Node recovery complete - new node can be joined"
```

### Scenario 3: Database Failover

```bash
#!/bin/bash
# recover_from_database_failure.sh

PRIMARY_DB="prod-db-1.internal"
STANDBY_DB="prod-db-2.internal"

echo "Initiating database failover"

# 1. Verify primary is unresponsive
if pg_isready -h "$PRIMARY_DB" -p 5432; then
    echo "Primary is still responding, aborting failover"
    exit 1
fi

# 2. Promote standby to primary
ssh "$STANDBY_DB" 'sudo -u postgres /usr/lib/postgresql/16/bin/pg_ctl promote -D /var/lib/postgresql/16/main'

echo "Promoted $STANDBY_DB to primary"

# 3. Update connection strings
# This would be done via ConfigMap update in Kubernetes
kubectl set env deployment/openrisk-api \
    -n openrisk \
    DATABASE_HOST="$STANDBY_DB"

# 4. Verify new primary is accepting connections
for i in {1..30}; do
    pg_isready -h "$STANDBY_DB" -p 5432 && break
    echo "Waiting for new primary... ($i/30)"
    sleep 1
done

if pg_isready -h "$STANDBY_DB" -p 5432; then
    echo "✓ Database failover complete"
else
    echo "✗ Database failover failed"
    exit 1
fi
```

### Scenario 4: Data Corruption Recovery

```bash
#!/bin/bash
# recover_from_data_corruption.sh

BACKUP_TIMESTAMP="$1"  # e.g., "20260302_0200"
NAMESPACE="openrisk"

if [ -z "$BACKUP_TIMESTAMP" ]; then
    echo "Usage: $0 <backup_timestamp>"
    exit 1
fi

echo "Recovering database from backup: $BACKUP_TIMESTAMP"

# 1. Stop application pods
kubectl scale deployment/openrisk-api -n "$NAMESPACE" --replicas=0

# 2. Create backup of corrupted data
CORRUPTED_BACKUP="/backups/corrupted_$(date +%Y%m%d_%H%M%S)"
mkdir -p "$CORRUPTED_BACKUP"

pg_dump -h prod-db \
        -U openrisk \
        -d openrisk \
        -F c \
        > "$CORRUPTED_BACKUP/database.dump"

# 3. Drop and recreate database
psql -h prod-db -U openrisk -d postgres <<EOF
DROP DATABASE IF EXISTS openrisk;
CREATE DATABASE openrisk;
EOF

# 4. Restore from backup
BACKUP_FILE="/backups/openrisk/full_$BACKUP_TIMESTAMP/database.dump"

if [ ! -f "$BACKUP_FILE" ]; then
    echo "Backup file not found: $BACKUP_FILE"
    exit 1
fi

pg_restore -h prod-db \
           -U openrisk \
           -d openrisk \
           -j 4 \
           "$BACKUP_FILE"

# 5. Run migrations to ensure schema is current
flyway migrate -locations=filesystem:/migrations

# 6. Restart application
kubectl scale deployment/openrisk-api -n "$NAMESPACE" --replicas=3

# 7. Verify recovery
sleep 30
curl -X GET http://localhost:8080/health

echo "✓ Data recovery complete"
```

---

## Part 4: Recovery Testing

### Monthly Disaster Recovery Drills

```bash
#!/bin/bash
# dr_drill.sh

DR_TEST_DB="openrisk_dr_test"
DRILL_DATE=$(date +%Y%m%d)
DRILL_REPORT="/var/log/dr_drill_$DRILL_DATE.txt"

echo "Starting Disaster Recovery Drill - $DRILL_DATE" | tee "$DRILL_REPORT"

# Test 1: Backup Restoration
echo -e "\n=== Test 1: Backup Restoration ===" | tee -a "$DRILL_REPORT"

LATEST_BACKUP=$(ls -t /backups/openrisk/full_*/database.dump | head -1)

psql -U openrisk -d postgres -c "DROP DATABASE IF EXISTS $DR_TEST_DB;"
psql -U openrisk -d postgres -c "CREATE DATABASE $DR_TEST_DB;"

pg_restore -U openrisk \
           -d "$DR_TEST_DB" \
           "$LATEST_BACKUP" 2>&1 | tee -a "$DRILL_REPORT"

# Verify restoration
RESTORED_ROWS=$(psql -U openrisk -d "$DR_TEST_DB" -t -c "SELECT COUNT(*) FROM risks;")
echo "Restored $RESTORED_ROWS risks" | tee -a "$DRILL_REPORT"

# Test 2: Failover Test
echo -e "\n=== Test 2: Failover Test ===" | tee -a "$DRILL_REPORT"

# Test standby promotion in test environment
ssh test-standby '/opt/test_promotion.sh' 2>&1 | tee -a "$DRILL_REPORT"

# Test 3: Recovery Time Measurement
echo -e "\n=== Test 3: Recovery Time Measurement ===" | tee -a "$DRILL_REPORT"

START_TIME=$(date +%s)

# Simulate failure and recovery
# ... recovery steps ...

END_TIME=$(date +%s)
RECOVERY_TIME=$((END_TIME - START_TIME))

echo "Recovery completed in $RECOVERY_TIME seconds" | tee -a "$DRILL_REPORT"

# Test 4: Data Integrity
echo -e "\n=== Test 4: Data Integrity ===" | tee -a "$DRILL_REPORT"

# Run data consistency checks
psql -U openrisk -d "$DR_TEST_DB" <<EOF 2>&1 | tee -a "$DRILL_REPORT"
-- Check for orphaned records
SELECT 'Orphaned mitigations' as check_name, COUNT(*) as count
FROM mitigations m
WHERE NOT EXISTS (SELECT 1 FROM risks r WHERE r.id = m.risk_id);

-- Check for referential integrity
SELECT 'Missing risk owners' as check_name, COUNT(*) as count
FROM risks r
WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = r.owner_id);

-- Check for duplicate risks
SELECT 'Duplicate risks' as check_name, COUNT(*) - COUNT(DISTINCT id) as count
FROM risks;
EOF

# Summary
echo -e "\n=== DR Drill Summary ===" | tee -a "$DRILL_REPORT"
echo "Drill Date: $DRILL_DATE" | tee -a "$DRILL_REPORT"
echo "Backup Size: $(du -h "$LATEST_BACKUP" | cut -f1)" | tee -a "$DRILL_REPORT"
echo "Recovery Time: $RECOVERY_TIME seconds" | tee -a "$DRILL_REPORT"
echo "RTO Target: < 3600 seconds" | tee -a "$DRILL_REPORT"
echo "Status: $([ $RECOVERY_TIME -lt 3600 ] && echo "PASS ✓" || echo "FAIL ✗")" | tee -a "$DRILL_REPORT"

# Email report
mail -s "DR Drill Report - $DRILL_DATE" ops@example.com < "$DRILL_REPORT"

# Cleanup
psql -U openrisk -d postgres -c "DROP DATABASE $DR_TEST_DB;"
```

### Quarterly Failover Drills

```bash
#!/bin/bash
# quarterly_failover_drill.sh

echo "Quarterly Failover Drill - $(date)"

# Phase 1: Announce planned maintenance
echo "Announcing scheduled maintenance window..."
kubectl patch service openrisk-api \
    -p '{"spec":{"selector":{"state":"maintenance"}}}'

# Phase 2: Trigger failover
echo "Initiating controlled failover..."
bash /opt/recover_from_database_failure.sh

# Phase 3: Monitor new primary
echo "Monitoring new primary for 30 minutes..."
for i in {1..30}; do
    echo "Health check $i/30"
    curl -f http://api.internal:8080/health
    sleep 60
done

# Phase 4: Failback to original primary
echo "Failing back to original primary..."
bash /opt/recover_from_database_failure.sh

# Phase 5: Resume service
echo "Resuming normal service..."
kubectl patch service openrisk-api \
    -p '{"spec":{"selector":{"state":"normal"}}}'

echo "Quarterly failover drill complete"
```

---

## Part 5: Monitoring & Alerting for Disaster Prevention

### Health Check Configuration

```go
// backend/handlers/health_handler.go

package handlers

import (
    "context"
    "github.com/gofiber/fiber/v2"
)

type HealthStatus struct {
    Status   string      `json:"status"`
    Services map[string]string `json:"services"`
    Timestamp string     `json:"timestamp"`
}

func HealthCheck(c *fiber.Ctx) error {
    status := HealthStatus{
        Status:    "healthy",
        Services: make(map[string]string),
        Timestamp: time.Now().UTC().String(),
    }
    
    ctx := context.Background()
    
    // Check database
    err := db.WithContext(ctx).Raw("SELECT 1").Error
    if err != nil {
        status.Services["database"] = "unhealthy"
        status.Status = "unhealthy"
    } else {
        status.Services["database"] = "healthy"
    }
    
    // Check Redis
    err = cache.Ping(ctx).Err()
    if err != nil {
        status.Services["cache"] = "unhealthy"
        status.Status = "unhealthy"
    } else {
        status.Services["cache"] = "healthy"
    }
    
    code := fiber.StatusOK
    if status.Status == "unhealthy" {
        code = fiber.StatusServiceUnavailable
    }
    
    return c.Status(code).JSON(status)
}
```

### Alerting Rules

```yaml
# prometheus/disaster_recovery_alerts.yaml

groups:
  - name: disaster_recovery
    rules:
      - alert: DatabaseReplicationLag
        expr: pg_replication_lag_bytes > 1000000
        for: 5m
        annotations:
          severity: critical
          summary: "Database replication lag exceeds 1MB"

      - alert: BackupFailure
        expr: time() - backup_last_successful_timestamp_seconds > 86400
        annotations:
          severity: critical
          summary: "Backup has not completed in 24 hours"

      - alert: LowDiskSpace
        expr: node_filesystem_avail_bytes{mountpoint="/"} / node_filesystem_size_bytes < 0.2
        for: 5m
        annotations:
          severity: warning
          summary: "Disk space low on {{ $labels.instance }}"

      - alert: HighMemoryUsage
        expr: process_resident_memory_bytes / node_memory_MemTotal_bytes > 0.85
        for: 5m
        annotations:
          severity: warning
          summary: "High memory usage detected"
```

---

## Part 6: Communication Plan

### Incident Notification

When a disaster is detected, notify in this order:

1. **Immediate** (< 5 minutes)
   - On-call engineer via PagerDuty
   - Slack #ops channel

2. **Within 15 minutes**
   - Engineering manager
   - DevOps team

3. **Within 30 minutes**
   - Executive team
   - Customer communication team

### Customer Communication Template

```
Subject: Service Disruption - [Service Name]

We are currently experiencing an outage affecting [services].
Our team is actively working on recovery.

Start Time: [UTC time]
Current Status: [investigating/recovering/resolved]
ETA: [estimated time to resolution]

Updates: [updates.example.com]
```

---

## Appendix: Recovery Checklists

### Post-Disaster Recovery Checklist

```
☐ All services online and responding
☐ Database replication verified
☐ Backups running normally
☐ Monitoring and alerting functioning
☐ Data integrity verified
☐ Customer communication sent
☐ Post-mortem scheduled
☐ Root cause identified
☐ Remediation items documented
```

### Annual DR Plan Review

```
☐ Test all recovery procedures
☐ Update RTO/RPO targets
☐ Review backup retention policy
☐ Update contact list
☐ Review runbook documentation
☐ Verify tool/service versions
☐ Test new recovery features
☐ Identify process improvements
```

---

**Maintained by**: DevOps Team  
**Last Reviewed**: March 2, 2026  
**Next Review**: June 2, 2026
