#!/bin/bash
# Verification script for Phase 5 Priority #4 completion

echo "=========================================="
echo "Phase 5 Priority #4 Verification"
echo "=========================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Counters
TOTAL=0
VERIFIED=0

# Function to check file
check_file() {
    local file=$1
    local description=$2
    TOTAL=$((TOTAL + 1))
    
    if [ -f "$file" ]; then
        lines=$(wc -l < "$file")
        echo -e "${GREEN}✓${NC} $description ($lines lines)"
        VERIFIED=$((VERIFIED + 1))
    else
        echo -e "${RED}✗${NC} $description - NOT FOUND"
    fi
}

echo "Checking Infrastructure Files..."
echo ""

# Backend code
check_file "backend/internal/handlers/cache_integration.go" "Cache Integration Layer"
check_file "backend/internal/cache/middleware.go" "Cache Middleware"
check_file "backend/internal/database/pool_config.go" "Connection Pool Config"

echo ""
echo "Checking Monitoring Stack..."
echo ""

# Monitoring files
check_file "deployment/docker-compose-monitoring.yaml" "Docker Compose Stack"
check_file "deployment/monitoring/prometheus.yml" "Prometheus Config"
check_file "deployment/monitoring/alerts.yml" "Alert Rules"
check_file "deployment/monitoring/alertmanager.yml" "AlertManager Config"
check_file "deployment/monitoring/grafana/provisioning/datasources/prometheus.yml" "Grafana Datasource"
check_file "deployment/monitoring/grafana/provisioning/dashboards/dashboard_provider.yml" "Grafana Dashboard Provider"
check_file "deployment/monitoring/grafana/dashboards/openrisk-performance.json" "Grafana Dashboard"

echo ""
echo "Checking Load Testing..."
echo ""

check_file "load_tests/cache_test.js" "k6 Load Test Script"
check_file "load_tests/README_LOAD_TESTING.md" "Load Testing Guide"

echo ""
echo "Checking Documentation..."
echo ""

check_file "docs/CACHING_INTEGRATION_GUIDE.md" "Caching Integration Guide"
check_file "docs/CACHE_INTEGRATION_IMPLEMENTATION.md" "Cache Implementation Guide"
check_file "docs/MONITORING_SETUP_GUIDE.md" "Monitoring Setup Guide"
check_file "docs/PHASE_5_QUICK_REFERENCE.md" "Quick Reference Card"
check_file "docs/PHASE_5_COMPLETION.md" "Phase 5 Completion Summary"
check_file "docs/PHASE_5_INDEX.md" "Complete Index"
check_file "docs/SESSION_SUMMARY.md" "Session Summary"

echo ""
echo "=========================================="
echo "Verification Results: $VERIFIED / $TOTAL"
echo "=========================================="

if [ $VERIFIED -eq $TOTAL ]; then
    echo -e "${GREEN}✓ All files present and verified!${NC}"
    echo ""
    echo "Next steps:"
    echo "1. Read: docs/PHASE_5_QUICK_REFERENCE.md"
    echo "2. Read: docs/CACHE_INTEGRATION_IMPLEMENTATION.md"
    echo "3. Start monitoring: docker-compose -f deployment/docker-compose-monitoring.yaml up -d"
    echo "4. Integrate cache into backend/cmd/server/main.go"
    echo "5. Run tests: cd load_tests && k6 run cache_test.js"
    exit 0
else
    echo -e "${RED}✗ Some files are missing!${NC}"
    echo "Missing: $((TOTAL - VERIFIED)) files"
    exit 1
fi
