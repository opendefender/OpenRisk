  README - Guides de dploiement crs

Les guides suivants ont t gnrs pour vous aider à dployer OpenRisk gratuitement :

---

  Fichiers de documentation crs

  QUICK_DEPLOY_GUIDE.md  START HERE
- Dure:  minutes
- Complexit: Facile
- Étapes rapides pour dployer sur Supabase, Redis Cloud, Render.com et Vercel
- Parfait pour avoir une dmo rapidement

  DEPLOYMENT_FREE_SERVICES.md  COMPREHENSIVE GUIDE
- Dure: - heures
- Complexit: Intermdiaire
- Guide complet avec explications dtailles
- Dpannage avanc
- Architecture et limitations

  INTEGRATION_GUIDE.md  TECHNICAL REFERENCE
- Configuration complte du frontend/backend
- Code d'exemple (axios, fetch)
- Tests d'intgration
- Debugging avanc (CORS, logs, etc.)

  DEPLOYMENT_CHECKLIST.txt  PROGRESS TRACKING
-  phases de dploiement
- Checkboxes pour suivre votre progression
- Rfrence rapide pour troubleshooting
- Temps estim:  minutes

---

  Fichiers de configuration crs

 . Dockerfile.render
- Dockerfile optimis pour Render.com
- Multi-stage build pour le backend Go
- Healthcheck inclus

 . frontend/vercel.json
- Configuration optimale pour Vercel
- Framework Vite prconfigur
- Build settings

 . frontend/.env.production
- Variables d'environnement pour production
- VITE_API_URL configure
- Prête à l'emploi

 . deploy-free-setup.sh
- Script Bash d'automation
- Gnre automatiquement les fichiers de configuration
- Explications interactives

---

  Plan d'action recommand

 Étape : Lire le guide rapide ( min)
bash
Ouvrir: QUICK_DEPLOY_GUIDE.md
Objectif: Comprendre l'architecture globale


 Étape : Prparer les services ( min)

. Crer compte Supabase
. Crer projet PostgreSQL
. Crer compte Redis Cloud
. Crer compte Render.com
. Crer compte Vercel


 Étape : Dployer le backend ( min)

. Render.com → New Web Service
. Connecter GitHub repo
. Configurer env variables (DB, Redis, JWT)
. Dployer et attendre


 Étape : Dployer le frontend ( min)

. Vercel → Import Project
. Root directory: frontend
. Ajouter VITE_API_URL
. Dployer


 Étape : Intgrer ( min)

. Mettre à jour CORS sur Render
. Tester API connectivity
. Login et valider features


TOTAL: ~ minutes ⏱

---

  Rcapitulatif des services gratuits

| Service | Plan Gratuit | Limites |
|---------|-------------|----------|
| Vercel | Illimit |  GB/mois bande passante |
| Render.com | Illimit | Sleep aprs  min inactivit |
| Supabase | Inclus |  MB DB,  GB transfert/mois |
| Redis Cloud | Inclus |  MB RAM |
| GitHub | Public repos | Gratuit illimiti |

Coût total: $./mois 

---

  Documentation supplmentaire disponible

 Dans le projet:
- docs/LOCAL_DEVELOPMENT.md - Setup local
- docs/API_REFERENCE.md - API documentation
- docs/BACKEND_ENDPOINTS_GUIDE.md - Endpoints
- docs/ADVANCED_PERMISSIONS.md - Permissions
- README.md - Vue d'ensemble gnrale

 En ligne:
- Backend Docs: https://openrisk-api.onrender.com/swagger (aprs dploiement)
- Repository: https://github.com/alex-dembele/OpenRisk

---

  Aide et support

 Documentation rapide par problme:

 CORS Error
→ INTEGRATION_GUIDE.md → Dpannage → CORS Error

 API non accessible
→ QUICK_DEPLOY_GUIDE.md → Dpannage rapide

 Database connection error
→ DEPLOYMENT_FREE_SERVICES.md → Troubleshooting

 Frontend ne charge pas
→ INTEGRATION_GUIDE.md → Diagnostic avanc

---

  Tips & Tricks

 Gnrer JWT_SECRET scuris
bash
openssl rand -base 


 Tester API depuis CLI
bash
curl https://openrisk-api.onrender.com/api/health

 Avec token
curl -H "Authorization: Bearer TOKEN" \
  https://openrisk-api.onrender.com/api/risks


 Eviter le sleep mode Render

Utiliser: https://uptimerobot.com (free)
Ping: https://openrisk-api.onrender.com/api/health
Interval: toutes les  minutes


 Monitoring basique

Render logs: https://render.com → Services → Logs
Vercel logs: https://vercel.com → Deployments → Logs


---

  Prochaines tapes aprs dploiement

.  Crer des utilisateurs de test
.  Ajouter des risques d'exemple
.  Tester les crations de mitigations
.  Valider les dashboards et graphiques
.  Vrifier les permissions et rles
.  Tester la gnration PDF
.  Partager le lien de dmo! 

---

  Fichiers cls du projet


OpenRisk/
 QUICK_DEPLOY_GUIDE.md           START HERE
 DEPLOYMENT_FREE_SERVICES.md     Full guide
 INTEGRATION_GUIDE.md             Technical
 DEPLOYMENT_CHECKLIST.txt        Progress
 Dockerfile.render               Docker
 deploy-free-setup.sh            Automation
 frontend/
    vercel.json                Vercel config
    .env.production            Env vars
 backend/
    go.mod
    cmd/server/main.go
 migrations/
 docs/
    API_REFERENCE.md
    LOCAL_DEVELOPMENT.md
    ...
 README.md


---

  Aprs le dploiement

Une fois votre lien de dmo obtenu :


https://openrisk-xxxx.vercel.app


Partagez-le:
-  Sur GitHub en description du repo
-  Sur votre portfolio
-  Avec les stakeholders
-  Sur les rseaux sociaux
-  Dans les CVs/portfolios

---

  Notes importantes

. Render.com sleep mode: Service s'endort aprs  min d'inactivit (free tier)
   - Solution: Utiliser uptimerobot.com pour des pings rguliers

. Supabase limitations:  MB de stockage
   - Archivez rgulirement les anciens risques

. Redis Cloud limitations:  MB de cache
   - Grez bien les sessions

. Vercel bandwidth:  GB/mois
   - Optimisez les images et utilisez le CDN

---

  Prêt à dployer?

. Ouvrez QUICK_DEPLOY_GUIDE.md
. Suivez les  tapes principales
. En cas de problme, consultez INTEGRATION_GUIDE.md
. Partagez votre dmo! 

---

Bon dploiement ! 

Questions? Consultez les guides crs ou la documentation du projet.
