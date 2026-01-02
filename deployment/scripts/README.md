# 🔧 Deployment Scripts

Automation scripts to help with OpenRisk deployment.

## 📄 Files

### deploy-free-setup.sh
Interactive setup assistant for free tier deployment.

**What it does**:
- ✅ Checks prerequisites (Git, Node.js, Go)
- ✅ Generates environment variable templates
- ✅ Creates configuration files
- ✅ Provides step-by-step guidance
- ✅ Generates JWT secrets

**Usage**:
```bash
bash deploy-free-setup.sh
```

**Output**:
- `frontend/.env.production` - Frontend env vars
- `.env.render` - Backend env vars template
- `DEPLOYMENT_RENDER_CONFIG.txt` - Render setup guide
- `DEPLOYMENT_VERCEL_CONFIG.txt` - Vercel setup guide
- Interactive prompts for account setup

### create-checklist.sh
Generates an interactive deployment checklist.

**What it does**:
- ✅ Creates `DEPLOYMENT_CHECKLIST.txt` file
- ✅ Includes 8 phases of deployment
- ✅ Checkboxes for progress tracking
- ✅ Troubleshooting reference

**Usage**:
```bash
bash create-checklist.sh
```

**Output**:
- `DEPLOYMENT_CHECKLIST.txt` - Interactive progress tracker

---

## 🚀 How to Use

### Option 1: Automated Setup (Recommended for beginners)

```bash
# Navigate to deployment directory
cd deployment/scripts

# Run the setup assistant
bash deploy-free-setup.sh

# Follow the interactive prompts
# Answer questions about GitHub repo status
# Choose whether to generate JWT secret
# Get guidance for each service
```

### Option 2: Manual Setup

```bash
# 1. Read the guides
cd deployment/guides
# Open README_DEPLOYMENT.txt

# 2. Create accounts on services
# 3. Manually configure environment variables
# 4. Deploy to each service
```

### Option 3: Hybrid Approach

```bash
# Use script to generate configs
bash deploy-free-setup.sh

# Manually follow the guides for details
# Reference generated files during setup
```

---

## 📋 Prerequisite Checks

The `deploy-free-setup.sh` script verifies:

✅ **Git** - Required for version control
✅ **Node.js** - Required for frontend build
✅ **Go** - Optional but recommended (for local testing)

If any are missing, you get helpful installation links.

---

## 🔐 Security Features

- Generates strong JWT secrets using `openssl`
- Creates `.env` file templates (not committing secrets)
- Provides guidance on secure configuration
- Explains how to store secrets in service dashboards

---

## 📝 Generated Files

### From deploy-free-setup.sh:

**`frontend/.env.production`**
```env
VITE_API_URL=https://openrisk-api.onrender.com
VITE_ENV=production
```

**`.env.render`** (template)
```env
DATABASE_URL=postgresql://postgres:PASSWORD@...
REDIS_URL=redis://default:PASSWORD@...
JWT_SECRET=generated-32-char-string
[... more variables ...]
```

**`DEPLOYMENT_RENDER_CONFIG.txt`**
- Step-by-step Render.com setup
- Environment variable mapping
- Expected deployment time

**`DEPLOYMENT_VERCEL_CONFIG.txt`**
- Step-by-step Vercel setup
- Environment variable mapping
- Expected deployment time

### From create-checklist.sh:

**`DEPLOYMENT_CHECKLIST.txt`**
- Phase 1: Preparation
- Phase 2: Infrastructure Setup
- Phase 3: Backend Deployment
- Phase 4: Frontend Deployment
- Phase 5: Integration & Verification
- Phase 6: Final Verification
- Phase 7: Production Readiness
- Phase 8: Launch & Sharing

---

## ⏱️ Timing

- **deploy-free-setup.sh**: 2-3 minutes
- **create-checklist.sh**: < 1 minute
- **Total setup time**: 3-5 minutes (very quick!)

---

## 💡 Tips

1. **Run before deployment**
   ```bash
   bash deploy-free-setup.sh
   ```
   This generates all config files you need.

2. **Generate JWT secret when prompted**
   - Creates a strong 32-character secret
   - You can use it in Render.com dashboard

3. **Follow generated guides**
   - Scripts create detailed setup instructions
   - Reference them while setting up services

4. **Create checklist for tracking**
   ```bash
   bash create-checklist.sh
   ```
   Use it to track your progress through 8 phases.

---

## 🔧 Technical Details

### Shell Requirements
- Bash 4.0+
- Standard Unix tools (echo, openssl, etc.)

### Platform Support
- ✅ Linux
- ✅ macOS
- ✅ Windows (with Git Bash or WSL)

### No External Dependencies
- All scripts use standard tools
- No npm packages required
- No additional installations needed

---

## 📞 Troubleshooting

**Script won't run**
```bash
# Make it executable
chmod +x deploy-free-setup.sh
chmod +x create-checklist.sh

# Run again
bash deploy-free-setup.sh
```

**Git not found**
```bash
# Install Git
# macOS: brew install git
# Ubuntu: sudo apt-get install git
# Windows: https://git-scm.com/download/win
```

**Node.js not found**
```bash
# Install Node.js
# https://nodejs.org/
```

**JWT secret generation fails**
```bash
# Manually generate instead
openssl rand -base64 32

# Copy the output and use in Render.com
```

---

## 🚀 Full Deployment Flow

1. **Run setup script** (2 min)
   ```bash
   bash deploy-free-setup.sh
   ```

2. **Create accounts** (10 min)
   - Supabase
   - Redis Cloud
   - Render.com
   - Vercel

3. **Deploy backend** (15 min)
   - Use generated DEPLOYMENT_RENDER_CONFIG.txt
   - Set environment variables
   - Deploy

4. **Deploy frontend** (10 min)
   - Use generated DEPLOYMENT_VERCEL_CONFIG.txt
   - Set environment variables
   - Deploy

5. **Test & integrate** (5 min)
   - Verify API responds
   - Test login
   - Check frontend loads

6. **Track progress** (5 min)
   - Use DEPLOYMENT_CHECKLIST.txt
   - Mark completed phases

---

## 📚 Related Documentation

- **Main Guide**: `../guides/README_DEPLOYMENT.txt`
- **Quick Start**: `../guides/QUICK_DEPLOY_GUIDE.md`
- **Configuration**: `../configs/README.md`
- **Docker**: `../docker/README.md`

---

## 🎯 Next Steps

1. Run the setup script:
   ```bash
   bash deploy-free-setup.sh
   ```

2. Follow the prompts

3. Reference generated files during deployment

4. Use DEPLOYMENT_CHECKLIST.txt to track progress

---

**Ready? Run: `bash deploy-free-setup.sh`** 🚀
