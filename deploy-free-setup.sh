#!/bin/bash

# ============================================================================
# OpenRisk Free Deployment Automation Script
# ============================================================================
# This script automates the setup for deploying OpenRisk to free services
# ============================================================================

set -e

echo "╔══════════════════════════════════════════════════════════════════════╗"
echo "║         OpenRisk Free Deployment Setup Assistant                     ║"
echo "╚══════════════════════════════════════════════════════════════════════╝"
echo ""

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ============================================================================
# Step 1: Verify Prerequisites
# ============================================================================
echo -e "${BLUE}[1/6]${NC} Checking prerequisites..."
echo ""

if ! command -v git &> /dev/null; then
    echo -e "${RED}✗ Git is not installed${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Git${NC} installed"

if ! command -v node &> /dev/null; then
    echo -e "${RED}✗ Node.js is not installed${NC}"
    echo "Install from: https://nodejs.org"
    exit 1
fi
echo -e "${GREEN}✓ Node.js${NC} ($(node --version))"

if ! command -v go &> /dev/null; then
    echo -e "${RED}✗ Go is not installed (optional but recommended)${NC}"
else
    echo -e "${GREEN}✓ Go${NC} ($(go version | awk '{print $3}'))"
fi

echo ""

# ============================================================================
# Step 2: GitHub Configuration
# ============================================================================
echo -e "${BLUE}[2/6]${NC} GitHub Configuration"
echo ""
echo "Make sure your repository is pushed to GitHub:"
echo "  https://github.com/your-username/OpenRisk"
echo ""

read -p "Is your repo pushed to GitHub? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo -e "${YELLOW}⚠ Please push your repository to GitHub first:${NC}"
    echo "  git remote add origin https://github.com/your-username/OpenRisk.git"
    echo "  git push -u origin main"
    exit 1
fi

echo -e "${GREEN}✓ Repository verified${NC}"
echo ""

# ============================================================================
# Step 3: Environment Variables Setup
# ============================================================================
echo -e "${BLUE}[3/6]${NC} Creating environment variable templates"
echo ""

# Frontend env
cat > frontend/.env.production << 'EOF'
# Production API endpoint
VITE_API_URL=https://openrisk-api.onrender.com

# Environment
VITE_ENV=production

# Analytics (optional)
# VITE_ANALYTICS_ID=your-id
EOF

echo -e "${GREEN}✓ Created${NC} frontend/.env.production"

# Backend env template
cat > .env.render << 'EOF'
# Database (from Supabase)
DATABASE_URL=postgresql://postgres:PASSWORD@host.supabase.co:5432/postgres

# Redis (from Redis Cloud)
REDIS_URL=redis://default:PASSWORD@host.redislabs.com:19999

# Server config
PORT=8080
ENVIRONMENT=production

# Security
JWT_SECRET=generate-a-strong-32-char-string-here-dont-use-this-example
JWT_EXPIRY=24h

# CORS - Update with your Vercel URL
CORS_ORIGINS=https://openrisk-xxxx.vercel.app,http://localhost:5173

# API
API_BASE_URL=https://openrisk-api.onrender.com

# Logging
LOG_LEVEL=info

# Database connection pool
DB_MAX_CONNECTIONS=10
DB_CONNECTION_TIMEOUT=5s
EOF

echo -e "${GREEN}✓ Created${NC} .env.render (template)"
echo ""

# ============================================================================
# Step 4: Installation Instructions
# ============================================================================
echo -e "${BLUE}[4/6]${NC} Service Registration Instructions"
echo ""

echo -e "${YELLOW}Please create accounts on these free services:${NC}"
echo ""
echo "1. ${BLUE}Supabase${NC} (PostgreSQL Database)"
echo "   → Go to: https://supabase.com"
echo "   → Sign up with GitHub"
echo "   → Create new project"
echo "   → Copy Connection String"
echo ""

echo "2. ${BLUE}Redis Cloud${NC} (Caching)"
echo "   → Go to: https://app.redislabs.com"
echo "   → Sign up with email"
echo "   → Create free database (30 MB tier)"
echo "   → Copy connection URL"
echo ""

echo "3. ${BLUE}Render.com${NC} (Backend API)"
echo "   → Go to: https://render.com"
echo "   → Sign up with GitHub"
echo "   → New Web Service"
echo "   → Connect your GitHub repo"
echo ""

echo "4. ${BLUE}Vercel${NC} (Frontend)"
echo "   → Go to: https://vercel.com"
echo "   → Sign up with GitHub"
echo "   → Import project"
echo "   → Root directory: frontend"
echo ""

# ============================================================================
# Step 5: Backend Configuration Template
# ============================================================================
echo -e "${BLUE}[5/6]${NC} Render.com Configuration for Backend"
echo ""

cat > DEPLOYMENT_RENDER_CONFIG.txt << 'EOF'
RENDER.COM CONFIGURATION
========================

Service Type: Web Service
Name: openrisk-api
Environment: Docker
Region: Frankfurt (or closest to you)

BUILD COMMAND:
docker build -f Dockerfile.render -t openrisk .

START COMMAND:
./server

ENVIRONMENT VARIABLES TO ADD:
- DATABASE_URL (from Supabase)
- REDIS_URL (from Redis Cloud)
- JWT_SECRET (generate 32+ char random string)
- CORS_ORIGINS=https://openrisk-xxxx.vercel.app
- API_BASE_URL=https://openrisk-api.onrender.com
- LOG_LEVEL=info
- ENVIRONMENT=production

EXPECTED DEPLOYMENT TIME: 3-5 minutes

RESULTING URL: https://openrisk-api.onrender.com
EOF

echo -e "${GREEN}✓ Created${NC} DEPLOYMENT_RENDER_CONFIG.txt"
echo ""

# ============================================================================
# Step 6: Vercel Configuration Template
# ============================================================================
echo -e "${BLUE}[6/6]${NC} Vercel Configuration for Frontend"
echo ""

cat > DEPLOYMENT_VERCEL_CONFIG.txt << 'EOF'
VERCEL CONFIGURATION
====================

Framework: Vite
Root Directory: frontend
Build Command: npm run build
Output Directory: dist
Node Version: 20.x

ENVIRONMENT VARIABLES TO ADD:
- VITE_API_URL=https://openrisk-api.onrender.com

EXPECTED DEPLOYMENT TIME: 2-3 minutes

RESULTING URL: https://openrisk-xxxx.vercel.app
EOF

echo -e "${GREEN}✓ Created${NC} DEPLOYMENT_VERCEL_CONFIG.txt"
echo ""

# ============================================================================
# Summary
# ============================================================================
echo ""
echo "╔══════════════════════════════════════════════════════════════════════╗"
echo "║                    ✅ Setup Complete!                                ║"
echo "╚══════════════════════════════════════════════════════════════════════╝"
echo ""
echo -e "${GREEN}Files created:${NC}"
echo "  • frontend/.env.production      (Vercel env config)"
echo "  • .env.render                   (Render env template)"
echo "  • Dockerfile.render             (Optimized for Render)"
echo "  • frontend/vercel.json          (Vercel configuration)"
echo "  • DEPLOYMENT_FREE_SERVICES.md   (Full deployment guide)"
echo ""
echo -e "${BLUE}Next steps:${NC}"
echo ""
echo "  1. Create Supabase project"
echo "     → https://supabase.com"
echo "     → Copy your CONNECTION STRING"
echo ""
echo "  2. Create Redis Cloud database"
echo "     → https://app.redislabs.com"
echo "     → Copy your CONNECTION URL"
echo ""
echo "  3. Deploy Backend on Render.com"
echo "     → https://render.com"
echo "     → Import from GitHub"
echo "     → Set environment variables (see DEPLOYMENT_RENDER_CONFIG.txt)"
echo ""
echo "  4. Deploy Frontend on Vercel"
echo "     → https://vercel.com"
echo "     → Import project from GitHub"
echo "     → Root: frontend"
echo "     → Set VITE_API_URL env var"
echo ""
echo "  5. Update CORS on Render"
echo "     → In Render env: CORS_ORIGINS=https://your-vercel-url.vercel.app"
echo ""
echo "  6. Test the application"
echo "     → Open https://openrisk-xxxx.vercel.app"
echo "     → Login with: admin@openrisk.local / admin123"
echo ""
echo -e "${BLUE}Documentation:${NC}"
echo "  → Read DEPLOYMENT_FREE_SERVICES.md for detailed instructions"
echo ""

# ============================================================================
# Optional: Generate JWT Secret
# ============================================================================
echo ""
read -p "Generate a random JWT_SECRET for .env.render? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    JWT_SECRET=$(openssl rand -base64 32)
    echo ""
    echo -e "${YELLOW}Generated JWT_SECRET:${NC}"
    echo -e "${GREEN}$JWT_SECRET${NC}"
    echo ""
    echo "Add this to your Render.com environment variables"
fi

echo ""
echo "🎉 Happy deploying!"
