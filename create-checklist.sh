#!/bin/bash

# ============================================================================
# OpenRisk Free Deployment Checklist
# ============================================================================
# Use this checklist to track your deployment progress
# ============================================================================

CHECKLIST_FILE="DEPLOYMENT_CHECKLIST.txt"

cat > "$CHECKLIST_FILE" << 'EOF'
╔══════════════════════════════════════════════════════════════════════════╗
║              OpenRisk Free Deployment Checklist                          ║
║              Complete all tasks to go live                               ║
╚══════════════════════════════════════════════════════════════════════════╝

PHASE 1: PREPARATION
═══════════════════════════════════════════════════════════════════════════

 □ GitHub Repository Setup
   □ Repository is public
   □ Code is pushed to main/master branch
   □ URL: https://github.com/your-username/OpenRisk
   
 □ Verify Project Structure
   □ /backend directory exists with Go files
   □ /frontend directory exists with React files
   □ Dockerfile.render exists in root
   □ frontend/vercel.json exists
   □ migrations/ directory with SQL files exists

 □ Git Configuration
   □ git remote -v shows correct GitHub URL
   □ All changes are committed
   □ No uncommitted changes


PHASE 2: INFRASTRUCTURE SETUP (Services)
═══════════════════════════════════════════════════════════════════════════

 □ Supabase (Database PostgreSQL)
   □ Account created: https://supabase.com
   □ Project created: openrisk-demo
   □ Database password saved securely
   □ Connection String copied
     Format: postgresql://postgres:PASSWORD@host.supabase.co:5432/postgres
   □ Region selected (closest to you)
   
 □ Redis Cloud (Cache)
   □ Account created: https://app.redislabs.com
   □ Free tier (30 MB) selected
   □ Database created: openrisk-cache
   □ Endpoint URL copied
     Format: redis://default:PASSWORD@host.redislabs.com:19999
   □ Default password saved

 □ Render.com (Backend API)
   □ Account created: https://render.com
   □ GitHub connected
   □ OpenRisk repository authorized
   
 □ Vercel (Frontend)
   □ Account created: https://vercel.com
   □ GitHub connected
   □ OpenRisk repository authorized


PHASE 3: BACKEND DEPLOYMENT (Render.com)
═══════════════════════════════════════════════════════════════════════════

 □ Create Web Service on Render
   □ Service name: openrisk-api
   □ Repository: OpenRisk
   □ Branch: main (or your branch)
   □ Build Command: docker build -f Dockerfile.render -t openrisk .
   □ Start Command: ./server
   □ Region: Frankfurt (or closest)
   □ Plan: Free tier

 □ Set Environment Variables (in Render Dashboard)
   □ DATABASE_URL = postgresql://postgres:PASSWORD@host.supabase.co:5432/postgres
   □ REDIS_URL = redis://default:PASSWORD@host.redislabs.com:19999
   □ JWT_SECRET = [32+ character random string]
   □ ENVIRONMENT = production
   □ PORT = 8080
   □ API_BASE_URL = https://openrisk-api.onrender.com
   □ LOG_LEVEL = info
   □ CORS_ORIGINS = https://openrisk-xxxx.vercel.app (add after Vercel setup)

 □ Deploy Backend
   □ Click "Deploy"
   □ Wait for build to complete (3-5 minutes)
   □ Check build logs for errors
   □ Service shows "Live"
   □ URL: https://openrisk-api.onrender.com

 □ Test Backend
   □ curl https://openrisk-api.onrender.com/api/health
   □ Response: {"status":"OK"}
   □ API Docs accessible: https://openrisk-api.onrender.com/swagger


PHASE 4: FRONTEND DEPLOYMENT (Vercel)
═══════════════════════════════════════════════════════════════════════════

 □ Import Project to Vercel
   □ Go to: https://vercel.com
   □ Click "Import Project"
   □ Select OpenRisk repository
   □ Confirm import

 □ Configure Vercel Project
   □ Root Directory: frontend
   □ Framework Preset: Vite
   □ Build Command: npm run build
   □ Output Directory: dist
   □ Node Version: 20.x

 □ Set Environment Variables (in Vercel)
   □ VITE_API_URL = https://openrisk-api.onrender.com

 □ Deploy Frontend
   □ Click "Deploy"
   □ Wait for build to complete (2-3 minutes)
   □ Check build logs for errors
   □ Deployment shows "Ready"
   □ URL: https://openrisk-xxxx.vercel.app

 □ Test Frontend
   □ Visit: https://openrisk-xxxx.vercel.app
   □ Page loads without errors
   □ No console errors in DevTools


PHASE 5: INTEGRATION & VERIFICATION
═══════════════════════════════════════════════════════════════════════════

 □ Update CORS on Render Backend
   □ Go to Render dashboard
   □ Edit openrisk-api service
   □ Update CORS_ORIGINS = https://openrisk-xxxx.vercel.app
   □ Manual deploy (or wait for auto-deploy)

 □ Test API Connectivity
   □ From frontend, try: GET /api/health
   □ Check Network tab in browser
   □ No CORS errors
   □ Response is successful

 □ Database Connectivity
   □ Frontend can fetch risks: GET /api/risks
   □ Frontend can view users
   □ No database errors in Render logs

 □ User Authentication
   □ Login with admin@openrisk.local / admin123
   □ Dashboard loads
   □ User session is maintained


PHASE 6: FINAL VERIFICATION
═══════════════════════════════════════════════════════════════════════════

 □ Application Functionality
   □ Can view dashboard
   □ Can create a risk
   □ Can add mitigation
   □ Can search/filter risks
   □ Charts render correctly
   □ Pagination works
   □ Sorting works

 □ Performance & Monitoring
   □ Frontend load time < 5 seconds
   □ API response time < 1 second
   □ No JavaScript errors in console
   □ Render health check is passing

 □ Documentation Updated
   □ README.md mentions the demo link
   □ DEPLOYMENT_FREE_SERVICES.md is referenced
   □ QUICK_DEPLOY_GUIDE.md is ready
   □ Add to github repo description


PHASE 7: PRODUCTION READINESS
═══════════════════════════════════════════════════════════════════════════

 □ Monitoring Setup
   □ Register at: https://uptimerobot.com (free)
   □ Add ping check to: https://openrisk-api.onrender.com/api/health
   □ Interval: every 14 minutes (prevent Render sleep)

 □ Backup Strategy
   □ Enable Supabase backups (automatic)
   □ Note backup frequency and retention
   □ Test restore procedure (optional)

 □ Logging & Debugging
   □ Check Render logs regularly
   □ Set up alerts for errors
   □ Monitor database query performance

 □ Security Checks
   □ Change default admin password (optional)
   □ JWT_SECRET is strong (32+ chars)
   □ HTTPS is enforced
   □ CORS is restrictive (only your domain)


PHASE 8: LAUNCH & SHARING
═══════════════════════════════════════════════════════════════════════════

 □ Create Demo Account (optional)
   □ Create new user for demo purposes
   □ Add sample risks
   □ Configure dashboard preferences
   □ Share credentials securely if needed

 □ Prepare Demo Materials
   □ Screenshot of dashboard
   □ Link to live demo: https://openrisk-xxxx.vercel.app
   □ API documentation: https://openrisk-xxxx.vercel.app/swagger
   □ GitHub repo link

 □ Announce the Demo
   □ Update GitHub README
   □ Share on social media (optional)
   □ Send to stakeholders
   □ Add to portfolio/website

 □ Monitor Initial Usage
   □ Check Render logs for errors
   □ Monitor database storage
   □ Check Redis memory usage
   □ Monitor Vercel bandwidth


TROUBLESHOOTING REFERENCE
═══════════════════════════════════════════════════════════════════════════

If you encounter issues, reference the solution in:
📖 DEPLOYMENT_FREE_SERVICES.md → Troubleshooting section
📖 QUICK_DEPLOY_GUIDE.md → Dépannage rapide section


FINAL CHECKLIST
═══════════════════════════════════════════════════════════════════════════

 □ Demo URL works: https://openrisk-xxxx.vercel.app
 □ Can login with credentials provided
 □ Backend API is responsive
 □ Database is connected
 □ All features tested
 □ No console errors
 □ Monitoring is set up
 □ Ready to share with others!


═══════════════════════════════════════════════════════════════════════════
ESTIMATED TIME: 45 minutes
ESTIMATED COST: $0.00
═══════════════════════════════════════════════════════════════════════════

✅ ALL DONE! Your demo is live! 🎉

Share your link: https://openrisk-xxxx.vercel.app

Questions? Check the documentation:
→ DEPLOYMENT_FREE_SERVICES.md
→ QUICK_DEPLOY_GUIDE.md
→ docs/LOCAL_DEVELOPMENT.md

EOF

echo "✅ Checklist created: $CHECKLIST_FILE"
echo ""
echo "📋 To use the checklist:"
echo "   1. Open the file in your editor"
echo "   2. Check off each box as you complete it"
echo "   3. Reference troubleshooting if needed"
echo ""
echo "📖 Files created for deployment:"
echo "   • DEPLOYMENT_CHECKLIST.txt"
echo "   • QUICK_DEPLOY_GUIDE.md"
echo "   • DEPLOYMENT_FREE_SERVICES.md"
echo "   • Dockerfile.render"
echo "   • frontend/vercel.json"
echo ""
