  Quick Start - D�ploiement gratuit en  minutes

 R�sum� rapide

Pour obtenir un lien de d�mo en  minutes avec z�ro frais, voici les  �tapes :

 ⃣ Base de donn�es PostgreSQL (Supabase) -  min

bash
. Allez sur https://supabase.com
. Sign up with GitHub
. New Project → openrisk-demo
. R�cup�rez: CONNECTION STRING (Settings → Database)
   Format: postgresql://postgres:PASSWORD@host.supabase.co:/postgres


 ⃣ Cache Redis (Redis Cloud) -  min

bash
. Allez sur https://app.redislabs.com
. Sign up → Free tier
. New Database →  MB
. R�cup�rez: redis-endpoint:port et PASSWORD
   Format: redis://default:PASSWORD@host.redislabs.com:


 ⃣ Backend API (Render.com) -  min

bash
. Allez sur https://render.com
. Sign up with GitHub → Connect repo OpenRisk
. New Web Service:
   - Name: openrisk-api
   - Environment: Docker
   - Build Command: docker build -f Dockerfile.render -t openrisk .
   
. Environment Variables:
   DATABASE_URL=postgresql://postgres:PASSWORD@...
   REDIS_URL=redis://default:PASSWORD@...
   JWT_SECRET=generez-une-cl�-de--chars
   CORS_ORIGINS=https://openrisk-xxxx.vercel.app (ajouter apr�s Vercel)
   API_BASE_URL=https://openrisk-api.onrender.com
   
. Deploy → Attendre - minutes
   URL r�sultante: https://openrisk-api.onrender.com


 ⃣ Frontend (Vercel) -  min

bash
. Allez sur https://vercel.com
. Sign up with GitHub → Import Project
. Configuration:
   - Select OpenRisk repository
   - Root Directory: frontend
   - Framework: Vite
   - Build Command: npm run build
   
. Environment Variable:
   VITE_API_URL=https://openrisk-api.onrender.com
   
. Deploy → Attendre - minutes
   URL r�sultante: https://openrisk-xxxx.vercel.app


---

  V�rification finale

. Testez l'API:
   bash
   curl https://openrisk-api.onrender.com/api/health
   

. Testez le frontend:
   
   https://openrisk-xxxx.vercel.app
   Email: admin@openrisk.local
   Password: admin
   

. Docs API Swagger:
   
   https://openrisk-api.onrender.com/swagger
   

---

  Cl�s d'acc�s par d�faut

Email: admin@openrisk.local  
Password: admin

---

  Stack de d�ploiement


┌─────────────────────────────────────┐
│   Vercel                            │
│   https://openrisk-xxxx.vercel.app  │
│   (Frontend React/Vite)             │
└──────────────┬──────────────────────┘
               │ HTTPS API calls
               ▼
┌─────────────────────────────────────┐
│   Render.com                        │
│   https://openrisk-api.onrender.com │
│   (Backend Go/Fiber)                │
└──────────────┬──────────────────────┘
               │
      ┌────────�────────┐
      ▼                  ▼
┌──────────────┐  ┌──────────────────┐
│   Supabase   │  │   Redis Cloud    │
│  PostgreSQL  │  │   Cache ( MB)  │
│  ( MB)    │  └──────────────────┘
└──────────────┘


---

  Limites gratuites à conna�tre

| Service | Limite | Contournement |
|---------|--------|---------------|
| Render.com | Sleep apr�s  min inactivit� | Utilisez uptimerobot.com (gratuit) pour ping |
| Vercel |  GB/mois bande passante | Optimisez images, utilisez CDN |
| Supabase |  MB DB +  GB transfert | Archivez les anciens risques |
| Redis Cloud |  MB RAM | Nettoyez le cache r�guli�rement |

---

  Commandes utiles

 G�n�rer un JWT_SECRET robuste
bash
openssl rand -base 


 Tester la connexion DB Supabase
bash
psql "postgresql://postgres:PASSWORD@host.supabase.co:/postgres" -c "SELECT "


 Tester Redis
bash
redis-cli -h host.redislabs.com -p  -a PASSWORD ping


---

  D�pannage rapide

  "CORS error - frontend cannot reach API"

→ Dans Render, v�rifier CORS_ORIGINS contient votre Vercel URL
→ Exemple: CORS_ORIGINS=https://openrisk-xxxx.vercel.app


  "Database connection error"

→ V�rifier DATABASE_URL dans Render env
→ Tester: psql "postgresql://..."


  "Render service goes to sleep"

→ Ajouter monitoring gratuit: https://uptimerobot.com
→ Ping toutes les  minutes: https://openrisk-api.onrender.com/api/health


  "Cannot login - admin user not created"

→ V�rifier que les migrations DB ont roul�
→ Dans Render logs, chercher "Database: Running Auto-Migrations"


---

 📚 Documentation compl�te

Pour les d�tails complets, consultez: DEPLOYMENT_FREE_SERVICES.md

---

 � Coût total

 $./mois

Tous les services utilis�s ont des plans gratuits g�n�reux !

---

  Prochaines �tapes apr�s le d�ploiement

.  Cr�ez des comptes utilisateur
.  Ajoutez des risques de test
.  Testez la cr�ation de mitigations
.  Validez les dashboards
.  Partagez le lien de d�mo : https://openrisk-xxxx.vercel.app

---

Bon d�ploiement ! 
