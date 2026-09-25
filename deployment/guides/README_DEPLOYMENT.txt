╔═══════════════════════════════════════════════════════════════════════════╗
║                                                                           ║
║              🚀 OPENRISK - DÉPLOIEMENT GRATUIT EN 45 MINUTES             ║
║                                                                           ║
╚═══════════════════════════════════════════════════════════════════════════╝

SITUATION ACTUELLE:
═══════════════════════════════════════════════════════════════════════════

✅ Vous avez une application Full-Stack complète:
   • Frontend: React 19 + Vite + TailwindCSS
   • Backend: Go 1.25.4 + Fiber v2 + PostgreSQL
   • Infrastructure Docker & Kubernetes ready
   • Base de code production-ready

❌ Manquant: Un lien de démo public

SOLUTION: Déployer sur des services 100% GRATUITS


SERVICES PROPOSÉS (100% GRATUITS):
═══════════════════════════════════════════════════════════════════════════

┌─ Frontend  ────────────── Vercel ───────────────────┐
│  React/Vite            https://vercel.com          │
│  → https://openrisk-xxxx.vercel.app                │
└──────────────────────────────────────────────────────┘
                           ⬇ HTTPS API
┌─ Backend   ────────────── Render.com ──────────────┐
│  Go/Fiber              https://render.com          │
│  → https://openrisk-api.onrender.com               │
└────────────────┬──────────────────────────┬─────────┘
                 │                          │
        ┌────────▼───────┐         ┌───────▼──────────┐
        │   Database     │         │     Cache        │
        │   Supabase     │         │  Redis Cloud     │
        │  PostgreSQL    │         │    30 MB RAM     │
        │  500 MB        │         │                  │
        └────────────────┘         └──────────────────┘


COÛT TOTAL: $0.00/MOIS 💰
═══════════════════════════════════════════════════════════════════════════


ROADMAP DE DÉPLOIEMENT (45 MINUTES):
═══════════════════════════════════════════════════════════════════════════

PHASE 1: PRÉPARER LA BASE DE DONNÉES (5 minutes)
──────────────────────────────────────────────────
[ ] 1. Allez sur https://supabase.com
[ ] 2. Sign up with GitHub → Create new project
     • Name: openrisk-demo
     • Region: closest to you
     • Password: save it!
[ ] 3. Copy CONNECTION STRING: postgresql://postgres:PASSWORD@...

PHASE 2: CONFIGURER LE CACHE (5 minutes)
──────────────────────────────────────────────────
[ ] 1. Allez sur https://app.redislabs.com
[ ] 2. Sign up → Free tier → New Database
     • Name: openrisk-cache
     • Plan: Free (30 MB)
[ ] 3. Copy: redis://default:PASSWORD@host.redislabs.com:19999

PHASE 3: DÉPLOYER LE BACKEND (15 minutes)
──────────────────────────────────────────────────
[ ] 1. Allez sur https://render.com
[ ] 2. Sign up with GitHub → Connect repo OpenRisk
[ ] 3. New Web Service:
     • Name: openrisk-api
     • Environment: Docker
     • Region: Frankfurt
     • Build: docker build -f Dockerfile.render -t openrisk .
     • Start: ./server

[ ] 4. Set ENVIRONMENT VARIABLES:
     DATABASE_URL=postgresql://postgres:PASSWORD@...
     REDIS_URL=redis://default:PASSWORD@...
     RSA_PRIVATE_KEY=<PEM private key, see scripts/generate_rsa_keys.sh>
     RSA_PUBLIC_KEY=<PEM public key>
     CORS_ORIGINS=https://openrisk-xxxx.vercel.app
     API_BASE_URL=https://openrisk-api.onrender.com
     PORT=8080
     ENVIRONMENT=production
     LOG_LEVEL=info

[ ] 5. Deploy → Wait 3-5 minutes
     ✓ Result: https://openrisk-api.onrender.com

PHASE 4: DÉPLOYER LE FRONTEND (10 minutes)
──────────────────────────────────────────────────
[ ] 1. Allez sur https://vercel.com
[ ] 2. Sign up with GitHub → Import Project
[ ] 3. Configuration:
     • Repository: OpenRisk
     • Root Directory: frontend
     • Framework: Vite
     • Build Command: npm run build

[ ] 4. Set ENVIRONMENT VARIABLE:
     VITE_API_URL=https://openrisk-api.onrender.com

[ ] 5. Deploy → Wait 2-3 minutes
     ✓ Result: https://openrisk-xxxx.vercel.app

PHASE 5: INTÉGRATION FINALE (5 minutes)
──────────────────────────────────────────────────
[ ] 1. Go back to Render backend service
[ ] 2. Update CORS_ORIGINS with your EXACT Vercel URL
[ ] 3. Manual Deploy OR wait for auto-deploy
[ ] 4. Test: curl https://openrisk-api.onrender.com/api/health
[ ] 5. Test frontend: https://openrisk-xxxx.vercel.app


RÉSULTAT FINAL:
═══════════════════════════════════════════════════════════════════════════

✅ Frontend: https://openrisk-xxxx.vercel.app
✅ Backend API: https://openrisk-api.onrender.com
✅ API Docs: https://openrisk-api.onrender.com/swagger
✅ Database: Supabase PostgreSQL (500 MB)
✅ Cache: Redis Cloud (30 MB)
✅ HTTPS: Automatic et gratuit
✅ CDN: Vercel global CDN


ACCÈS DE DÉMO:
═══════════════════════════════════════════════════════════════════════════

Email: admin@opendefender.io
Mot de passe: celui de INITIAL_ADMIN_PASSWORD, ou celui généré au premier
démarrage dans /app/secrets/initial_admin_password. Aucun mot de passe par
défaut n'existe.


DOCUMENTATION FOURNIE:
═══════════════════════════════════════════════════════════════════════════

📖 QUICK_DEPLOY_GUIDE.md (30 min read)
   → Résumé rapide des 4 étapes
   → Parfait pour commencer

📖 DEPLOYMENT_FREE_SERVICES.md (comprehensive)
   → Guide complet avec explications détaillées
   → Limitations gratuites et contournements
   → Troubleshooting avancé

📖 INTEGRATION_GUIDE.md (technical)
   → Configuration frontend/backend
   → Code d'exemple (axios, fetch)
   → Debugging CORS et API connectivity

✅ DEPLOYMENT_CHECKLIST.txt
   → 8 phases avec checkboxes
   → Progress tracking
   → Time estimates

⚙️ Dockerfile.render
   → Dockerfile optimisé pour Render

📋 frontend/vercel.json
   → Configuration Vercel prête à l'emploi


FICHIERS DE CONFIGURATION CRÉÉS:
═══════════════════════════════════════════════════════════════════════════

✅ Dockerfile.render
   → Optimisé pour Render.com
   → Multi-stage build
   → Healthcheck inclus

✅ frontend/vercel.json
   → Configuration Vercel (Vite)
   → Build settings optimisés

✅ frontend/.env.production
   → Variables d'environnement production
   → VITE_API_URL préconfiguré


ÉTAPES DÉTAILLÉES:
═══════════════════════════════════════════════════════════════════════════

ÉTAPE 1: SUPABASE (5 min)
────────────────────────
1. https://supabase.com → Sign up with GitHub
2. New Project → Name: openrisk-demo
3. Wait for database initialization
4. Settings → Database → Copy CONNECTION STRING
5. Format: postgresql://postgres:PASSWORD@host.supabase.co:5432/postgres
6. Sauvegardez cette string!

ÉTAPE 2: REDIS CLOUD (5 min)
──────────────────────────────
1. https://app.redislabs.com → Sign up
2. New Subscription → Free → Continue
3. Cloud: AWS, Region: Frankfurt (ou proche)
4. Database: openrisk-cache
5. Connectivity → Public endpoint → Copy URL
6. Format: redis://default:PASSWORD@host.redislabs.com:19999
7. Copy password from "Default user password"

ÉTAPE 3: RENDER.COM BACKEND (15 min)
──────────────────────────────────────
1. https://render.com → Sign up with GitHub
2. Select "Connect" next to OpenRisk repository
3. Web Service:
   - Name: openrisk-api
   - Environment: Docker
   - Region: Frankfurt (or nearest)
   - Build Command: docker build -f Dockerfile.render -t openrisk .
   - Start Command: ./server

4. Add Environment Variables:
   KEY                     VALUE
   ─────────────────────   ─────────────────────────────────
   DATABASE_URL            postgresql://postgres:PASSWORD@...
   REDIS_URL               redis://default:PASSWORD@...
   RSA_PRIVATE_KEY         [PEM, scripts/generate_rsa_keys.sh]
   RSA_PUBLIC_KEY          [PEM]
   API_BASE_URL            https://openrisk-api.onrender.com
   CORS_ORIGINS            https://openrisk-xxxx.vercel.app (later)
   PORT                    8080
   ENVIRONMENT             production
   LOG_LEVEL               info

5. Click "Create Web Service"
6. Wait for build (status: "Live")
7. Your URL: https://openrisk-api.onrender.com

ÉTAPE 4: VERCEL FRONTEND (10 min)
──────────────────────────────────
1. https://vercel.com → Sign up with GitHub
2. Import Project → Select OpenRisk
3. Configuration:
   - Framework: Vite
   - Root Directory: frontend
   - Build Command: npm run build
   - Output Directory: dist

4. Environment Variable:
   VITE_API_URL = https://openrisk-api.onrender.com

5. Click "Deploy"
6. Wait for deployment (status: "Ready")
7. Your URL: https://openrisk-xxxx.vercel.app

ÉTAPE 5: INTÉGRATION (5 min)
──────────────────────────────
1. Go back to Render dashboard
2. Click openrisk-api service
3. Update environment variables:
   CORS_ORIGINS = https://openrisk-xxxx.vercel.app (EXACT URL!)
4. Manual Deploy OR wait 5 minutes for auto-redeploy
5. Test: curl https://openrisk-api.onrender.com/api/health


TESTS DE VALIDATION:
═══════════════════════════════════════════════════════════════════════════

TEST 1: Backend Health
──────────────────────
curl https://openrisk-api.onrender.com/api/health
Expected: {"status":"OK"}

TEST 2: Frontend Load
──────────────────────
Visit: https://openrisk-xxxx.vercel.app
Expected: Login page loads

TEST 3: Authentication
──────────────────────
1. Open: https://openrisk-xxxx.vercel.app
2. Email: admin@opendefender.io
3. Password: INITIAL_ADMIN_PASSWORD, or the one generated in
   /app/secrets/initial_admin_password on first boot
4. Expected: Dashboard loads

TEST 4: API Integration
──────────────────────
Open DevTools Console:
fetch('https://openrisk-api.onrender.com/api/health')
  .then(r => r.json())
  .then(d => console.log('✅', d))
  .catch(e => console.error('❌', e))


PROBLÈMES COURANTS & SOLUTIONS:
═══════════════════════════════════════════════════════════════════════════

❌ "CORS error - frontend cannot reach API"
✅ Solution: Vérifier CORS_ORIGINS dans Render contient votre URL Vercel
   → Render Dashboard → Environment Variables
   → CORS_ORIGINS = https://openrisk-xxxx.vercel.app (EXACT!)
   → Redeploy

❌ "Database connection error"
✅ Solution: Vérifier DATABASE_URL dans Render
   → Test: psql "postgresql://postgres:PASSWORD@..."
   → Vérifier password et host

❌ "Cannot login - 401 Unauthorized"
✅ Solution: la paire de clés RS256 a changé
   → Vérifier RSA_PRIVATE_KEY / RSA_PUBLIC_KEY sur Render
   → Redeploy

❌ "API not responding / Network error"
✅ Solution: Render service dormant (free tier)
   → Attendre 30-60 secondes
   → Service se réveille
   → Ou configurer uptimerobot.com pour ping réguliers

❌ "Frontend is blank / Loading forever"
✅ Solution: Vérifier VITE_API_URL dans Vercel
   → Redeploy Vercel
   → Vérifier console DevTools


ÉVITER LE SLEEP MODE RENDER (IMPORTANT):
═══════════════════════════════════════════════════════════════════════════

Render free tier endort le service après 15 min d'inactivité.
Solution gratuite: Utiliser uptimerobot.com

1. Go to: https://uptimerobot.com
2. Sign up (free account)
3. New Monitor:
   - Friendly Name: OpenRisk API
   - Monitor Type: HTTP(s)
   - URL: https://openrisk-api.onrender.com/api/health
   - Interval: Every 14 minutes
   - Save

✅ Service ne dormira plus!


LIMITES GRATUITES À CONNAÎTRE:
═══════════════════════════════════════════════════════════════════════════

Service         │ Limite                    │ Contournement
────────────────┼───────────────────────────┼──────────────────────
Vercel          │ 100 GB/mois bande pass.   │ Optimiser images
Render.com      │ Sleep 15 min inactivité   │ uptimerobot.com
Supabase DB     │ 500 MB + 2 GB/mois trans. │ Archiver les risques
Redis Cloud     │ 30 MB RAM                 │ Gérer les sessions


PROCHAINES ÉTAPES APRÈS DÉPLOIEMENT:
═══════════════════════════════════════════════════════════════════════════

1. Tester les features:
   ✅ Create/Read/Update/Delete risks
   ✅ Add mitigations
   ✅ Search/Filter
   ✅ PDF export
   ✅ Dashboards & Charts
   ✅ User management

2. Optimiser:
   ✅ Change default admin password
   ✅ Configure backups Supabase
   ✅ Set up monitoring
   ✅ Monitor database usage

3. Partager:
   ✅ Update GitHub README with demo link
   ✅ Share on socials
   ✅ Add to portfolio
   ✅ Send to stakeholders


DOCUMENTATION DISPONIBLE:
═══════════════════════════════════════════════════════════════════════════

DEPLOYMENT_START_HERE.md
   → Guide d'orientation (ce document)

QUICK_DEPLOY_GUIDE.md
   → Guide rapide (30 minutes)
   → Résumé des 4 étapes principales

DEPLOYMENT_FREE_SERVICES.md
   → Documentation complète et détaillée
   → Toutes les étapes avec explications
   → FAQ et troubleshooting avancé

INTEGRATION_GUIDE.md
   → Guide d'intégration technique
   → Configuration API frontend/backend
   → Debugging CORS et connectivity
   → Code examples (axios, fetch)

DEPLOYMENT_CHECKLIST.txt
   → Checklist interactive avec checkboxes
   → 8 phases de déploiement
   → Progress tracking


FICHIERS MODIFIÉS/CRÉÉS:
═══════════════════════════════════════════════════════════════════════════

✅ Dockerfile.render          - Dockerfile optimisé pour Render
✅ frontend/vercel.json       - Configuration Vercel (Vite)
✅ frontend/.env.production   - Variables d'env production
✅ deploy-free-setup.sh       - Script d'automation (bash)
✅ QUICK_DEPLOY_GUIDE.md      - Guide rapide
✅ DEPLOYMENT_FREE_SERVICES.md - Guide complet
✅ INTEGRATION_GUIDE.md       - Guide technique
✅ DEPLOYMENT_CHECKLIST.txt   - Checklist interactive
✅ DEPLOYMENT_START_HERE.md   - Ce guide


POINTS CLÉS À RETENIR:
═══════════════════════════════════════════════════════════════════════════

1. Tous les services utilisés sont 100% GRATUITS
2. Temps total de déploiement: ~45 minutes
3. Coût total: $0.00/mois
4. Vous obtenez un lien de démo public: https://openrisk-xxxx.vercel.app

Architecture:
├── Frontend (Vercel) → HTTPS
├── Backend (Render.com) → HTTPS
├── Database (Supabase) → PostgreSQL
└── Cache (Redis Cloud) → Redis

CI/CD:
├── Push to GitHub → Auto-deploy on Vercel/Render
└── No manual deployment needed


SUPPORT:
═══════════════════════════════════════════════════════════════════════════

Question?           → Consulter la documentation appropriée
CORS error?         → INTEGRATION_GUIDE.md
Database issue?     → DEPLOYMENT_FREE_SERVICES.md
General help?       → QUICK_DEPLOY_GUIDE.md
Tracking progress?  → DEPLOYMENT_CHECKLIST.txt


═══════════════════════════════════════════════════════════════════════════

🚀 READY TO DEPLOY? 

👉 Start with: QUICK_DEPLOY_GUIDE.md

You've got this! 💪

═══════════════════════════════════════════════════════════════════════════
