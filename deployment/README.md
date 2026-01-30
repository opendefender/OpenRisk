  Deployment Directory

Complete guide and configuration files for deploying OpenRisk using free services.

  Directory Structure


deployment/
  guides/                     Complete deployment guides
    README_DEPLOYMENT.txt      Main guide ( min) - START HERE
    QUICK_DEPLOY_GUIDE.md      Fast track ( min)
    DEPLOYMENT_FREE_SERVICES.md  Detailed instructions
    INTEGRATION_GUIDE.md       Technical reference
    ARCHITECTURE_DEPLOYMENT.md  Visual diagrams & architecture
    DEPLOYMENT_START_HERE.md   Orientation & overview

  docker/                     Docker configuration
    Dockerfile.render          Optimized for Render.com

  configs/                    Configuration files
    .env.production            Production environment variables

  scripts/                    Automation scripts
    deploy-free-setup.sh       Interactive setup assistant
    create-checklist.sh        Generate deployment checklist

  _START_HERE.txt           Overview & quick links
  INDEX.md                    File navigation & reference
  GIT_COMMANDS.md             Git commands for deployment
  DEPLOYMENT_CHECKLIST.txt    Progress tracking ( phases)


---

  Quick Start

 Option : Fast Track ( minutes)
bash
cd deployment/guides
Open: QUICK_DEPLOY_GUIDE.md


 Option : Complete Guide ( minutes)
bash
cd deployment/guides
Open: README_DEPLOYMENT.txt


 Option : Automated Setup
bash
cd deployment/scripts
bash deploy-free-setup.sh


---

  Guide Selection

| I want to... | Read this |
|---|---|
| Get started quickly | guides/QUICK_DEPLOY_GUIDE.md |
| Complete deployment | guides/README_DEPLOYMENT.txt |
| Understand architecture | guides/ARCHITECTURE_DEPLOYMENT.md |
| Debug issues | guides/INTEGRATION_GUIDE.md |
| Track progress | DEPLOYMENT_CHECKLIST.txt |
| Find files | INDEX.md |
| Understand Git steps | GIT_COMMANDS.md |

---

  Services Stack (Free)


Vercel (Frontend)
  ↓ HTTPS
Render.com (Backend - Docker)
  ↓
Supabase (PostgreSQL -  MB)
Redis Cloud (Cache -  MB)


Total Cost: $./month

---

  Timeline

| Phase | Duration | Task |
|-------|----------|------|
|  |  min | Create service accounts |
|  |  min | Deploy backend (Render) |
|  |  min | Deploy frontend (Vercel) |
|  |  min | Integration testing |
|  |  min | Validation & sharing |
| Total |  min | From zero to demo link |

---

  Default Credentials

After deployment:
- Email: admin@openrisk.local
- Password: admin

Change these after initial setup!

---

  Files Overview

 Guides Directory
- README_DEPLOYMENT.txt - Your main deployment guide with all  phases
- QUICK_DEPLOY_GUIDE.md - Abbreviated version for -minute deployment
- DEPLOYMENT_FREE_SERVICES.md - Detailed instructions for each service
- INTEGRATION_GUIDE.md - Frontend/Backend API integration & debugging
- ARCHITECTURE_DEPLOYMENT.md - Diagrams, data flow, security architecture
- DEPLOYMENT_START_HERE.md - Overview of all deployment resources

 Root Level
- _START_HERE.txt - Entry point with key information
- INDEX.md - Navigation hub for all files
- GIT_COMMANDS.md - Git push commands to save your work
- DEPLOYMENT_CHECKLIST.txt - Track progress through  phases

 Docker
- Dockerfile.render - Production-optimized Docker image for Render.com

 Config
- .env.production - Environment variables template

 Scripts
- deploy-free-setup.sh - Interactive setup that checks prerequisites
- create-checklist.sh - Generate an interactive checklist

---

  Environment Setup

 Frontend (.env.production in frontend/)
env
VITE_API_URL=https://openrisk-api.onrender.com
VITE_ENV=production


 Backend (set in Render.com dashboard)
env
DATABASE_URL=postgresql://postgres:PASSWORD@...
REDIS_URL=redis://default:PASSWORD@...
JWT_SECRET=your--character-secret-key
CORS_ORIGINS=https://openrisk-xxxx.vercel.app
API_BASE_URL=https://openrisk-api.onrender.com
PORT=
ENVIRONMENT=production
LOG_LEVEL=info


---

  Expected Result

After following the guides:

 Frontend: https://openrisk-xxxx.vercel.app  
 API: https://openrisk-api.onrender.com  
 Docs: https://openrisk-api.onrender.com/swagger  
 Auto-Deploy: Push to GitHub → Auto-deploy everywhere  
 HTTPS: Automatic certificate management  
 CDN: Global content delivery network  

---

  Important Notes

. Render Sleep Mode - Services sleep after  min inactivity (free tier)
   - Solution: Use uptimerobot.com (free) to ping every  minutes

. Supabase Limits -  MB storage,  GB/month transfer
   - Archive old risks periodically

. Redis Cache -  MB RAM
   - Implement session cleanup

. Vercel Bandwidth -  GB/month included
   - Optimize images if needed

---

  Next Steps

. Choose your learning style:
   - Fast? → Open guides/QUICK_DEPLOY_GUIDE.md
   - Thorough? → Open guides/README_DEPLOYMENT.txt
   - Interactive? → Run scripts/deploy-free-setup.sh

. Create accounts:
   - https://supabase.com (Database)
   - https://app.redislabs.com (Cache)
   - https://render.com (Backend)
   - https://vercel.com (Frontend)

. Follow the  phases

. Share your demo link!

---

  Support

All answers are in the guides. Quick reference:

- CORS Error? → See guides/INTEGRATION_GUIDE.md
- API Issue? → See guides/INTEGRATION_GUIDE.md
- Lost? → See _START_HERE.txt
- Tracking progress? → Use DEPLOYMENT_CHECKLIST.txt

---

  Version Info

- Created: December , 
- OpenRisk Version: ..
- Framework: React  + Go ..
- Status: Production-ready deployment package

---

Ready to deploy? Open _START_HERE.txt or guides/README_DEPLOYMENT.txt → Let's go! 
