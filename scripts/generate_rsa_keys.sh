#!/usr/bin/env bash
# 
# Script: generate_rsa_keys.sh
# Purpose: Generate RSA 4096-bit key pair for JWT RS256 signing in OpenRisk
# Usage: bash scripts/generate_rsa_keys.sh
#
# Outputs:
#   - private.pem (KEEP SECURE — never commit)
#   - public.pem (safe to commit and distribute)
#
# Environment Variables (set after script runs):
#   - RSA_PRIVATE_KEY_PATH: absolute path to private.pem
#   - RSA_PUBLIC_KEY_PATH: absolute path to public.pem
#   - RSA_PRIVATE_KEY: (optional) inline PEM content of private key
#   - RSA_PUBLIC_KEY: (optional) inline PEM content of public key

set -e

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
SECRETS_DIR="${PROJECT_ROOT}/secrets"
PRIVATE_KEY_FILE="${SECRETS_DIR}/private.pem"
PUBLIC_KEY_FILE="${SECRETS_DIR}/public.pem"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}═══════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}🔐 OpenRisk JWT RS256 Key Pair Generator${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════════════${NC}"
echo ""

# Create secrets directory if not exists
if [ ! -d "$SECRETS_DIR" ]; then
    mkdir -p "$SECRETS_DIR"
    echo -e "${YELLOW}✓ Created directory: ${SECRETS_DIR}${NC}"
else
    echo -e "${YELLOW}ℹ Using existing directory: ${SECRETS_DIR}${NC}"
fi

# Check if keys already exist
if [ -f "$PRIVATE_KEY_FILE" ] && [ -f "$PUBLIC_KEY_FILE" ]; then
    echo ""
    read -p "Keys already exist. Regenerate? (y/N): " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo -e "${YELLOW}⊘ Skipped. Using existing keys.${NC}"
        exit 0
    fi
    rm -f "$PRIVATE_KEY_FILE" "$PUBLIC_KEY_FILE"
    echo -e "${YELLOW}✓ Old keys removed${NC}"
fi

# Generate private key (RSA 4096 bits)
echo ""
echo -e "${YELLOW}Generating RSA 4096-bit private key...${NC}"
openssl genrsa -out "$PRIVATE_KEY_FILE" 4096 2>/dev/null
echo -e "${GREEN}✓ Private key generated${NC}"

# Extract public key
echo -e "${YELLOW}Extracting public key...${NC}"
openssl rsa -in "$PRIVATE_KEY_FILE" -pubout -out "$PUBLIC_KEY_FILE" 2>/dev/null
echo -e "${GREEN}✓ Public key extracted${NC}"

# Set restrictive permissions on private key
chmod 600 "$PRIVATE_KEY_FILE"
echo -e "${GREEN}✓ Private key permissions set to 600 (read/write owner only)${NC}"

# Display paths
echo ""
echo -e "${GREEN}═══════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}✓ Key Generation Complete${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════════════${NC}"
echo ""
echo -e "${YELLOW}📍 Private Key (SECURE):${NC}"
echo "   ${PRIVATE_KEY_FILE}"
echo ""
echo -e "${YELLOW}📍 Public Key (shareable):${NC}"
echo "   ${PUBLIC_KEY_FILE}"
echo ""

# Generate environment variable instructions
echo -e "${YELLOW}📋 Environment Variables to Set:${NC}"
echo ""
echo "For local development (.env):"
echo "───────────────────────────────────────────────────────────────"
cat <<EOF
# RSA Keys for JWT RS256 Authentication
RSA_PRIVATE_KEY_PATH="${PRIVATE_KEY_FILE}"
RSA_PUBLIC_KEY_PATH="${PUBLIC_KEY_FILE}"
EOF
echo "───────────────────────────────────────────────────────────────"
echo ""

echo -e "${YELLOW}For production (inline env vars):${NC}"
echo "───────────────────────────────────────────────────────────────"
echo "RSA_PRIVATE_KEY='$(sed 's/$/\\n/' "$PRIVATE_KEY_FILE" | tr -d '\n')'"
echo ""
echo "RSA_PUBLIC_KEY='$(sed 's/$/\\n/' "$PUBLIC_KEY_FILE" | tr -d '\n')'"
echo "───────────────────────────────────────────────────────────────"
echo ""

# Verify keys
echo -e "${YELLOW}Verifying keys...${NC}"
if openssl rsa -in "$PRIVATE_KEY_FILE" -check -noout 2>/dev/null | grep -q "RSA key ok"; then
    echo -e "${GREEN}✓ Private key verification: OK${NC}"
else
    echo -e "${RED}✗ Private key verification: FAILED${NC}"
    exit 1
fi

if openssl rsa -in "$PUBLIC_KEY_FILE" -pubin -check -noout 2>/dev/null | grep -q "RSA key ok"; then
    echo -e "${GREEN}✓ Public key verification: OK${NC}"
else
    echo -e "${RED}✗ Public key verification: FAILED${NC}"
    exit 1
fi

# Verify key pair match (keys are mathematically related)
if openssl rsa -in "$PRIVATE_KEY_FILE" -pubout 2>/dev/null | diff - "$PUBLIC_KEY_FILE" > /dev/null; then
    echo -e "${GREEN}✓ Key pair verification: OK (public key matches private key)${NC}"
else
    echo -e "${RED}✗ Key pair verification: FAILED${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}═══════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}🎉 All checks passed! Ready to use.${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════════════${NC}"
echo ""
echo -e "${YELLOW}⚠️  IMPORTANT SECURITY REMINDERS:${NC}"
echo "  1. Never commit private.pem to version control"
echo "  2. Rotate keys annually for production"
echo "  3. Store private.pem in a secure vault (HashiCorp, AWS Secrets, etc.)"
echo "  4. Backup keys in a secure location"
echo "  5. Use different key pairs for dev/staging/production"
echo ""
echo "   3. Restart the backend server"
echo ""
