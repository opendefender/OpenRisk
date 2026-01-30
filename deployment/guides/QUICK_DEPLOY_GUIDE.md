  Quick Start - Dploiement gratuit en  minutes

 Rsum rapide

Pour obtenir un lien de dmo en  minutes avec zro frais, voici les  tapes :

 ⃣ Base de donnes PostgreSQL (Supabase) -  min

bash
. Allez sur https://supabase.com
. Sign up with GitHub
. New Project → openrisk-demo
. Rcuprez: CONNECTION STRING (Settings → Database)
   Format: postgresql://postgres:PASSWORD@host.supabase.co:/postgres


 ⃣ Cache Redis (Redis Cloud) -  min

bash
. Allez sur https://app.redislabs.com
. Sign up → Free tier
. New Database →  MB
. Rcuprez: redis-endpoint:port et PASSWORD
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
   JWT_SECRET=generez-une-cl-de--chars
   CORS_ORIGINS=https://openrisk-xxxx.vercel.app (ajouter aprs Vercel)
   API_BASE_URL=https://openrisk-api.onrender.com
   
. Deploy → Attendre - minutes
   URL rsultante: https://openrisk-api.onrender.com


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
   URL rsultante: https://openrisk-xxxx.vercel.app


---

  Vrification finale

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

  Cls d'accs par dfaut

Email: admin@openrisk.local  
Password: admin

---

  Stack de dploiement



   Vercel                            
   https://openrisk-xxxx.vercel.app  
   (Frontend React/Vite)             

                HTTPS API calls
               

   Render.com                        
   https://openrisk-api.onrender.com 
   (Backend Go/Fiber)                

               
      
                        
  
   Supabase        Redis Cloud    
  PostgreSQL       Cache ( MB)  
  ( MB)      



---

  Limites gratuites à connatre

| Service | Limite | Contournement |
|---------|--------|---------------|
| Render.com | Sleep aprs  min inactivit | Utilisez uptimerobot.com (gratuit) pour ping |
| Vercel |  GB/mois bande passante | Optimisez images, utilisez CDN |
| Supabase |  MB DB +  GB transfert | Archivez les anciens risques |
| Redis Cloud |  MB RAM | Nettoyez le cache rgulirement |

---

  Commandes utiles

 Gnrer un JWT_SECRET robuste
bash
openssl rand -base 


 Tester la connexion DB Supabase
bash
psql "postgresql://postgres:PASSWORD@host.supabase.co:/postgres" -c "SELECT "


 Tester Redis
bash
redis-cli -h host.redislabs.com -p  -a PASSWORD ping


---

  Dpannage rapide

  "CORS error - frontend cannot reach API"

→ Dans Render, vrifier CORS_ORIGINS contient votre Vercel URL
→ Exemple: CORS_ORIGINS=https://openrisk-xxxx.vercel.app


  "Database connection error"

→ Vrifier DATABASE_URL dans Render env
→ Tester: psql "postgresql://..."


  "Render service goes to sleep"

→ Ajouter monitoring gratuit: https://uptimerobot.com
→ Ping toutes les  minutes: https://openrisk-api.onrender.com/api/health


  "Cannot login - admin user not created"

→ Vrifier que les migrations DB ont roul
→ Dans Render logs, chercher "Database: Running Auto-Migrations"


---

  Documentation complte

Pour les dtails complets, consultez: DEPLOYMENT_FREE_SERVICES.md

---

  Coût total

 $./mois

Tous les services utiliss ont des plans gratuits gnreux !

---

  Prochaines tapes aprs le dploiement

.  Crez des comptes utilisateur
.  Ajoutez des risques de test
.  Testez la cration de mitigations
.  Validez les dashboards
.  Partagez le lien de dmo : https://openrisk-xxxx.vercel.app

---

Bon dploiement ! 
