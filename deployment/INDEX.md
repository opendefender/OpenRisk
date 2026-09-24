---
title: "OpenRisk - Free Deployment Package"
description: "Complete guide to deploy OpenRisk on free services in 45 minutes"
date: "2025-12-25"
---

# 🚀 OpenRisk Free Deployment Package

**Complete solution to deploy OpenRisk for FREE and get a public demo link in 45 minutes**

---

## 📚 Quick Navigation

### 🎯 Start Here (Choose Your Path)

| I want to... | Time | Read this |
|---|---|---|
| Get a demo ASAP | 30 min | [QUICK_DEPLOY_GUIDE.md](./QUICK_DEPLOY_GUIDE.md) |
| Understand everything | 1-2 hours | [DEPLOYMENT_FREE_SERVICES.md](./DEPLOYMENT_FREE_SERVICES.md) |
| See diagrams & architecture | 15 min | [ARCHITECTURE_DEPLOYMENT.md](./ARCHITECTURE_DEPLOYMENT.md) |
| Debug technical issues | 30 min | [INTEGRATION_GUIDE.md](./INTEGRATION_GUIDE.md) |
| Track my progress | 45 min | [DEPLOYMENT_CHECKLIST.txt](./DEPLOYMENT_CHECKLIST.txt) |

---

## 📖 Documentation Files

### ⭐ [README_DEPLOYMENT.txt](./README_DEPLOYMENT.txt)
**Your main deployment guide**
- Complete 45-minute walkthrough
- All 5 deployment phases
- Common problems & solutions
- Validation tests
- **Status**: ✅ Ready to use

### 🚀 [QUICK_DEPLOY_GUIDE.md](./QUICK_DEPLOY_GUIDE.md)
**Fast track to demo link (30 minutes)**
- Abbreviated step-by-step
- Perfect for quick start
- Stack diagram
- Default credentials
- **Status**: ✅ Ready to use

### 📖 [DEPLOYMENT_FREE_SERVICES.md](./DEPLOYMENT_FREE_SERVICES.md)
**Comprehensive detailed guide**
- Every service explained
- Step-by-step instructions
- Configuration best practices
- Advanced troubleshooting
- **Status**: ✅ Ready to use

### 🔌 [INTEGRATION_GUIDE.md](./INTEGRATION_GUIDE.md)
**Technical integration reference**
- Frontend configuration
- Backend configuration
- Code examples (axios, fetch)
- API testing
- CORS debugging
- **Status**: ✅ Ready to use

### 🏗️ [ARCHITECTURE_DEPLOYMENT.md](./ARCHITECTURE_DEPLOYMENT.md)
**Visual architecture reference**
- Component diagrams
- Data flow examples
- Technology matrix
- Security architecture
- **Status**: ✅ Ready to use

### ✅ [DEPLOYMENT_CHECKLIST.txt](./DEPLOYMENT_CHECKLIST.txt)
**Progress tracking checklist**
- 8 phases with checkboxes
- Track completion
- Troubleshooting reference
- **Status**: ✅ Ready to use

### 📍 [DEPLOYMENT_START_HERE.md](./DEPLOYMENT_START_HERE.md)
**Orientation & file guide**
- Overview of all docs
- File descriptions
- Which to read when
- **Status**: ✅ Ready to use

---

## ⚙️ Configuration Files

### [Dockerfile.render](./Dockerfile.render)
- Optimized for Render.com
- Multi-stage build
- Health checks included
- **Status**: ✅ Ready to deploy

### [frontend/vercel.json](./frontend/vercel.json)
- Vercel configuration
- Framework: Vite
- Build settings optimized
- **Status**: ✅ Ready to deploy

### [frontend/.env.production](./frontend/.env.production)
- Production environment variables
- VITE_API_URL configured
- **Status**: ✅ Ready to use

---

## 🔧 Helper Scripts

### [deploy-free-setup.sh](./deploy-free-setup.sh)
- Interactive setup assistant
- Prerequisite checks
- Config generation
- **Usage**: `bash deploy-free-setup.sh`

### [create-checklist.sh](./create-checklist.sh)
- Generate interactive checklist
- **Usage**: `bash create-checklist.sh`

---

## 📊 Services Stack

```
Frontend:  Vercel (Free)
           ↓ HTTPS API
Backend:   Render.com (Free)
           ↓
Database:  Supabase (Free - 500 MB)
Cache:     Redis Cloud (Free - 30 MB)

Total Cost: $0.00/month
```

---

## 🎯 Deployment Timeline

| Phase | Task | Duration |
|-------|------|----------|
| 1️⃣ | Setup services (accounts) | 10 min |
| 2️⃣ | Deploy backend (Render) | 15 min |
| 3️⃣ | Deploy frontend (Vercel) | 10 min |
| 4️⃣ | Integration testing | 5 min |
| 5️⃣ | Final validation | 5 min |
| | **TOTAL** | **45 min** |

---

## 🔑 Quick Reference

### First sign-in
```
Email:    admin@opendefender.io
Password: INITIAL_ADMIN_PASSWORD, or the one generated on first boot
          in /app/secrets/initial_admin_password (never logged)
```

### Result URLs
```
Frontend: https://openrisk-xxxx.vercel.app
API: https://openrisk-api.onrender.com
Docs: https://openrisk-api.onrender.com/swagger
```

### Services to Create Accounts
1. **Supabase** (Database) → https://supabase.com
2. **Redis Cloud** (Cache) → https://app.redislabs.com
3. **Render.com** (Backend) → https://render.com
4. **Vercel** (Frontend) → https://vercel.com

---

## 🚀 Next Steps

1. **Choose your path** (see table above)
2. **Read the appropriate guide**
3. **Create service accounts**
4. **Follow the deployment phases**
5. **Get your demo link!**

---

## 📞 Troubleshooting Quick Links

| Issue | Solution |
|-------|----------|
| CORS Error | See [INTEGRATION_GUIDE.md](./INTEGRATION_GUIDE.md) - CORS debugging |
| API not responding | See [QUICK_DEPLOY_GUIDE.md](./QUICK_DEPLOY_GUIDE.md) - Dépannage rapide |
| Database error | See [DEPLOYMENT_FREE_SERVICES.md](./DEPLOYMENT_FREE_SERVICES.md) - Troubleshooting |
| Frontend blank | See [INTEGRATION_GUIDE.md](./INTEGRATION_GUIDE.md) - Diagnostic avancé |

---

## ✨ What You'll Get

✅ Public demo link: `https://openrisk-xxxx.vercel.app`  
✅ Live API: `https://openrisk-api.onrender.com`  
✅ HTTPS everywhere  
✅ Global CDN  
✅ Automatic backups  
✅ Auto-deploy on git push  
✅ $0.00/month cost  

---

## 📋 Files in This Package

```
OpenRisk/
├── 📖 Documentation/
│   ├── README_DEPLOYMENT.txt ..................... Main guide
│   ├── QUICK_DEPLOY_GUIDE.md ..................... 30-min quick start
│   ├── DEPLOYMENT_FREE_SERVICES.md .............. Complete guide
│   ├── INTEGRATION_GUIDE.md ...................... Technical reference
│   ├── ARCHITECTURE_DEPLOYMENT.md ............... Visual diagrams
│   ├── DEPLOYMENT_START_HERE.md ................. Orientation
│   ├── DEPLOYMENT_CHECKLIST.txt ................. Progress tracker
│   └── INDEX.md (this file) ..................... Navigation
│
├── ⚙️ Configuration/
│   ├── Dockerfile.render ......................... For Render.com
│   ├── frontend/vercel.json ..................... For Vercel
│   ├── frontend/.env.production ................. Env variables
│   └── GIT_COMMANDS.md .......................... Push to GitHub
│
├── 🔧 Scripts/
│   ├── deploy-free-setup.sh ..................... Interactive setup
│   ├── create-checklist.sh ...................... Generate checklist
│   └── FILES_CREATED.sh ......................... This summary
│
└── 🏠 Project Root/
    ├── README.md ................................ Original project docs
    ├── backend/ ................................. Go backend
    ├── frontend/ ................................ React frontend
    ├── migrations/ .............................. DB migrations
    └── docs/ .................................... Other documentation
```

---

## 💡 Key Points to Remember

1. **Completely FREE** - All services used have generous free tiers
2. **45 minutes** - From zero to deployed demo
3. **No credit card needed** - For initial free tier
4. **Auto-deploy** - Push to GitHub → auto-deploy everywhere
5. **HTTPS included** - All endpoints are secure
6. **Global CDN** - Frontend loads fast worldwide

---

## ⚠️ Important Limitations

| Service | Limit | Solution |
|---------|-------|----------|
| Render | Sleep 15 min | Use uptimerobot.com |
| Supabase | 500 MB | Archive old data |
| Redis Cloud | 30 MB | Manage sessions |
| Vercel | 100 GB/mo | Optimize images |

---

## 🎓 Learning Resources

- **Vercel Docs**: https://vercel.com/docs
- **Render Docs**: https://render.com/docs
- **Supabase Docs**: https://supabase.com/docs
- **Redis Docs**: https://redis.io/docs

---

## 📝 Version Info

- **Created**: December 25, 2025
- **OpenRisk Version**: 1.0.4
- **Go Version**: 1.25.4
- **React Version**: 19.2.0
- **Status**: Ready for deployment

---

## 🤝 Support

Questions or issues?

1. Check the [Troubleshooting Quick Links](#troubleshooting-quick-links) above
2. Read the [Documentation Files](#-documentation-files) relevant to your issue
3. Consult [INTEGRATION_GUIDE.md](./INTEGRATION_GUIDE.md) for technical help
4. Review [ARCHITECTURE_DEPLOYMENT.md](./ARCHITECTURE_DEPLOYMENT.md) for understanding

---

## 🎉 Ready?

**👉 [Start with README_DEPLOYMENT.txt →](./README_DEPLOYMENT.txt)**

Or choose your path from the [Quick Navigation](#-quick-navigation) table above.

---

**Good luck with your deployment! You've got this! 🚀**
