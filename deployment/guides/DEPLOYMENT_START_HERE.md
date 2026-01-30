 ğŸ“– README - Guides de dÃploiement crÃÃs

Les guides suivants ont ÃtÃ gÃnÃrÃs pour vous aider Ã  dÃployer OpenRisk gratuitement :

---

 ğŸ“š Fichiers de documentation crÃÃs

  QUICK_DEPLOY_GUIDE.md â­ START HERE
- DurÃe:  minutes
- ComplexitÃ: Facile
- Ã‰tapes rapides pour dÃployer sur Supabase, Redis Cloud, Render.com et Vercel
- Parfait pour avoir une dÃmo rapidement

 ğŸ“– DEPLOYMENT_FREE_SERVICES.md ğŸ“– COMPREHENSIVE GUIDE
- DurÃe: - heures
- ComplexitÃ: IntermÃdiaire
- Guide complet avec explications dÃtaillÃes
- DÃpannage avancÃ
- Architecture et limitations

 ğŸ”Œ INTEGRATION_GUIDE.md ğŸ”Œ TECHNICAL REFERENCE
- Configuration complÃte du frontend/backend
- Code d'exemple (axios, fetch)
- Tests d'intÃgration
- Debugging avancÃ (CORS, logs, etc.)

  DEPLOYMENT_CHECKLIST.txt  PROGRESS TRACKING
-  phases de dÃploiement
- Checkboxes pour suivre votre progression
- RÃfÃrence rapide pour troubleshooting
- Temps estimÃ:  minutes

---

 ğŸ“ Fichiers de configuration crÃÃs

 . Dockerfile.render
- Dockerfile optimisÃ pour Render.com
- Multi-stage build pour le backend Go
- Healthcheck inclus

 . frontend/vercel.json
- Configuration optimale pour Vercel
- Framework Vite prÃconfigurÃ
- Build settings

 . frontend/.env.production
- Variables d'environnement pour production
- VITE_API_URL configurÃe
- PrÃªte Ã  l'emploi

 . deploy-free-setup.sh
- Script Bash d'automation
- GÃnÃre automatiquement les fichiers de configuration
- Explications interactives

---

  Plan d'action recommandÃ

 Ã‰tape : Lire le guide rapide ( min)
bash
Ouvrir: QUICK_DEPLOY_GUIDE.md
Objectif: Comprendre l'architecture globale


 Ã‰tape : PrÃparer les services ( min)

. CrÃer compte Supabase
. CrÃer projet PostgreSQL
. CrÃer compte Redis Cloud
. CrÃer compte Render.com
. CrÃer compte Vercel


 Ã‰tape : DÃployer le backend ( min)

. Render.com â†’ New Web Service
. Connecter GitHub repo
. Configurer env variables (DB, Redis, JWT)
. DÃployer et attendre


 Ã‰tape : DÃployer le frontend ( min)

. Vercel â†’ Import Project
. Root directory: frontend
. Ajouter VITE_API_URL
. DÃployer


 Ã‰tape : IntÃgrer ( min)

. Mettre Ã  jour CORS sur Render
. Tester API connectivity
. Login et valider features


TOTAL: ~ minutes â±

---

  RÃcapitulatif des services gratuits

| Service | Plan Gratuit | Limites |
|---------|-------------|----------|
| Vercel | IllimitÃ |  GB/mois bande passante |
| Render.com | IllimitÃ | Sleep aprÃs  min inactivitÃ |
| Supabase | Inclus |  MB DB,  GB transfert/mois |
| Redis Cloud | Inclus |  MB RAM |
| GitHub | Public repos | Gratuit illimitiÃ |

CoÃ»t total: $./mois ğŸ’

---

  Documentation supplÃmentaire disponible

 Dans le projet:
- docs/LOCAL_DEVELOPMENT.md - Setup local
- docs/API_REFERENCE.md - API documentation
- docs/BACKEND_ENDPOINTS_GUIDE.md - Endpoints
- docs/ADVANCED_PERMISSIONS.md - Permissions
- README.md - Vue d'ensemble gÃnÃrale

 En ligne:
- Backend Docs: https://openrisk-api.onrender.com/swagger (aprÃs dÃploiement)
- Repository: https://github.com/alex-dembele/OpenRisk

---

 ğŸ†˜ Aide et support

 Documentation rapide par problÃme:

 CORS Error
â†’ INTEGRATION_GUIDE.md â†’ DÃpannage â†’ CORS Error

 API non accessible
â†’ QUICK_DEPLOY_GUIDE.md â†’ DÃpannage rapide

 Database connection error
â†’ DEPLOYMENT_FREE_SERVICES.md â†’ Troubleshooting

 Frontend ne charge pas
â†’ INTEGRATION_GUIDE.md â†’ Diagnostic avancÃ

---

  Tips & Tricks

 GÃnÃrer JWT_SECRET sÃcurisÃ
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

Render logs: https://render.com â†’ Services â†’ Logs
Vercel logs: https://vercel.com â†’ Deployments â†’ Logs


---

  Prochaines Ãtapes aprÃs dÃploiement

.  CrÃer des utilisateurs de test
.  Ajouter des risques d'exemple
.  Tester les crÃations de mitigations
.  Valider les dashboards et graphiques
.  VÃrifier les permissions et rÃles
.  Tester la gÃnÃration PDF
.  Partager le lien de dÃmo! 

---

 ğŸ“ Fichiers clÃs du projet


OpenRisk/
â”œâ”€â”€ QUICK_DEPLOY_GUIDE.md          â­ START HERE
â”œâ”€â”€ DEPLOYMENT_FREE_SERVICES.md    ğŸ“– Full guide
â”œâ”€â”€ INTEGRATION_GUIDE.md            ğŸ”Œ Technical
â”œâ”€â”€ DEPLOYMENT_CHECKLIST.txt        Progress
â”œâ”€â”€ Dockerfile.render              ğŸ³ Docker
â”œâ”€â”€ deploy-free-setup.sh            Automation
â”œâ”€â”€ frontend/
â”‚   â”œâ”€â”€ vercel.json                Vercel config
â”‚   â””â”€â”€ .env.production            Env vars
â”œâ”€â”€ backend/
â”‚   â”œâ”€â”€ go.mod
â”‚   â””â”€â”€ cmd/server/main.go
â”œâ”€â”€ migrations/
â”œâ”€â”€ docs/
â”‚   â”œâ”€â”€ API_REFERENCE.md
â”‚   â”œâ”€â”€ LOCAL_DEVELOPMENT.md
â”‚   â””â”€â”€ ...
â””â”€â”€ README.md


---

  AprÃs le dÃploiement

Une fois votre lien de dÃmo obtenu :


https://openrisk-xxxx.vercel.app


Partagez-le:
-  Sur GitHub en description du repo
-  Sur votre portfolio
-  Avec les stakeholders
-  Sur les rÃseaux sociaux
-  Dans les CVs/portfolios

---

  Notes importantes

. Render.com sleep mode: Service s'endort aprÃs  min d'inactivitÃ (free tier)
   - Solution: Utiliser uptimerobot.com pour des pings rÃguliers

. Supabase limitations:  MB de stockage
   - Archivez rÃguliÃrement les anciens risques

. Redis Cloud limitations:  MB de cache
   - GÃrez bien les sessions

. Vercel bandwidth:  GB/mois
   - Optimisez les images et utilisez le CDN

---

  PrÃªt Ã  dÃployer?

. Ouvrez QUICK_DEPLOY_GUIDE.md
. Suivez les  Ãtapes principales
. En cas de problÃme, consultez INTEGRATION_GUIDE.md
. Partagez votre dÃmo! 

---

Bon dÃploiement ! 

Questions? Consultez les guides crÃÃs ou la documentation du projet.
