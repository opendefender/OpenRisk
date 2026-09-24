# 🚀 Deployment Directory

Complete guide and configuration files for deploying OpenRisk using free services.

## 📁 Directory Structure

```
deployment/
├── 📖 guides/                    # Complete deployment guides
│   ├── README_DEPLOYMENT.txt     # Main guide (45 min) - START HERE
│   ├── QUICK_DEPLOY_GUIDE.md     # Fast track (30 min)
│   ├── DEPLOYMENT_FREE_SERVICES.md # Detailed instructions
│   ├── INTEGRATION_GUIDE.md      # Technical reference
│   ├── ARCHITECTURE_DEPLOYMENT.md # Visual diagrams & architecture
│   └── DEPLOYMENT_START_HERE.md  # Orientation & overview
│
├── 🐳 docker/                    # Docker configuration
│   └── Dockerfile.render         # Optimized for Render.com
│
├── ⚙️ configs/                   # Configuration files
│   └── .env.production           # Production environment variables
│
├── 🔧 scripts/                   # Automation scripts
│   ├── deploy-free-setup.sh      # Interactive setup assistant
│   └── create-checklist.sh       # Generate deployment checklist
│
├── 📖 00_START_HERE.txt          # Overview & quick links
├── 📋 INDEX.md                   # File navigation & reference
├── 📝 GIT_COMMANDS.md            # Git commands for deployment
└── ✅ DEPLOYMENT_CHECKLIST.txt   # Progress tracking (8 phases)
```

---

## 🎯 Quick Start

### Option 1: Fast Track (30 minutes)
```bash
cd deployment/guides
Open: QUICK_DEPLOY_GUIDE.md
```

### Option 2: Complete Guide (45 minutes)
```bash
cd deployment/guides
Open: README_DEPLOYMENT.txt
```

### Option 3: Automated Setup
```bash
cd deployment/scripts
bash deploy-free-setup.sh
```

---

## 📚 Guide Selection

| I want to... | Read this |
|---|---|
| Get started quickly | `guides/QUICK_DEPLOY_GUIDE.md` |
| Complete deployment | `guides/README_DEPLOYMENT.txt` |
| Understand architecture | `guides/ARCHITECTURE_DEPLOYMENT.md` |
| Debug issues | `guides/INTEGRATION_GUIDE.md` |
| Track progress | `DEPLOYMENT_CHECKLIST.txt` |
| Find files | `INDEX.md` |
| Understand Git steps | `GIT_COMMANDS.md` |

---

## 🚀 Services Stack (Free)

```
Vercel (Frontend)
  ↓ HTTPS
Render.com (Backend - Docker)
  ↓
Supabase (PostgreSQL - 500 MB)
Redis Cloud (Cache - 30 MB)
```

**Total Cost: $0.00/month**

---

## ⏱️ Timeline

| Phase | Duration | Task |
|-------|----------|------|
| 1 | 10 min | Create service accounts |
| 2 | 15 min | Deploy backend (Render) |
| 3 | 10 min | Deploy frontend (Vercel) |
| 4 | 5 min | Integration testing |
| 5 | 5 min | Validation & sharing |
| **Total** | **45 min** | **From zero to demo link** |

---

## 🎯 First sign-in

There are no default credentials. The first boot against an empty database
creates `admin@opendefender.io` with the password in `INITIAL_ADMIN_PASSWORD`.
Leave it empty and the backend generates one and writes it to
`INITIAL_ADMIN_PASSWORD_FILE` (default `/app/secrets/initial_admin_password`, mode 0600). The password is never written to the logs.

Change it after the first sign-in, then delete that file.

---

## 📖 Files Overview

### Guides Directory
- **README_DEPLOYMENT.txt** - Your main deployment guide with all 5 phases
- **QUICK_DEPLOY_GUIDE.md** - Abbreviated version for 30-minute deployment
- **DEPLOYMENT_FREE_SERVICES.md** - Detailed instructions for each service
- **INTEGRATION_GUIDE.md** - Frontend/Backend API integration & debugging
- **ARCHITECTURE_DEPLOYMENT.md** - Diagrams, data flow, security architecture
- **DEPLOYMENT_START_HERE.md** - Overview of all deployment resources

### Root Level
- **00_START_HERE.txt** - Entry point with key information
- **INDEX.md** - Navigation hub for all files
- **GIT_COMMANDS.md** - Git push commands to save your work
- **DEPLOYMENT_CHECKLIST.txt** - Track progress through 8 phases

### Docker
- **Dockerfile.render** - Production-optimized Docker image for Render.com

### Config
- **.env.production** - Environment variables template

### Scripts
- **deploy-free-setup.sh** - Interactive setup that checks prerequisites
- **create-checklist.sh** - Generate an interactive checklist

---

## 🔑 Environment Setup

### Frontend (.env.production in frontend/)
```env
VITE_API_URL=https://openrisk-api.onrender.com
VITE_ENV=production
```

### Backend (set in Render.com dashboard)
```env
DATABASE_URL=postgresql://postgres:PASSWORD@...
REDIS_URL=redis://default:PASSWORD@...
RSA_PRIVATE_KEY=<PEM private key, see scripts/generate_rsa_keys.sh>
RSA_PUBLIC_KEY=<PEM public key>
CORS_ORIGINS=https://openrisk-xxxx.vercel.app
API_BASE_URL=https://openrisk-api.onrender.com
PORT=8080
ENVIRONMENT=production
LOG_LEVEL=info
```

---

## 🌟 Expected Result

After following the guides:

✅ **Frontend**: https://openrisk-xxxx.vercel.app  
✅ **API**: https://openrisk-api.onrender.com  
✅ **Docs**: https://openrisk-api.onrender.com/swagger  
✅ **Auto-Deploy**: Push to GitHub → Auto-deploy everywhere  
✅ **HTTPS**: Automatic certificate management  
✅ **CDN**: Global content delivery network  

---

## ⚠️ Important Notes

1. **Render Sleep Mode** - Services sleep after 15 min inactivity (free tier)
   - Solution: Use uptimerobot.com (free) to ping every 14 minutes

2. **Supabase Limits** - 500 MB storage, 2 GB/month transfer
   - Archive old risks periodically

3. **Redis Cache** - 30 MB RAM
   - Implement session cleanup

4. **Vercel Bandwidth** - 100 GB/month included
   - Optimize images if needed

---

## 🚀 Next Steps

1. **Choose your learning style**:
   - Fast? → Open `guides/QUICK_DEPLOY_GUIDE.md`
   - Thorough? → Open `guides/README_DEPLOYMENT.txt`
   - Interactive? → Run `scripts/deploy-free-setup.sh`

2. **Create accounts**:
   - https://supabase.com (Database)
   - https://app.redislabs.com (Cache)
   - https://render.com (Backend)
   - https://vercel.com (Frontend)

3. **Follow the 5 phases**

4. **Share your demo link!**

---

## 📞 Support

All answers are in the guides. Quick reference:

- **CORS Error?** → See `guides/INTEGRATION_GUIDE.md`
- **API Issue?** → See `guides/INTEGRATION_GUIDE.md`
- **Lost?** → See `00_START_HERE.txt`
- **Tracking progress?** → Use `DEPLOYMENT_CHECKLIST.txt`

---

## 📝 Version Info

- **Created**: December 25, 2025
- **OpenRisk Version**: 1.0.4
- **Framework**: React 19 + Go 1.25.4
- **Status**: Production-ready deployment package

---

**Ready to deploy? Open `00_START_HERE.txt` or `guides/README_DEPLOYMENT.txt` → Let's go! 🚀**
