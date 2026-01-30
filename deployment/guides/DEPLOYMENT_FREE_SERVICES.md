  Guide de dploiement gratuit - OpenRisk

Ce guide vous explique comment dployer OpenRisk totalement gratuitement pour obtenir un lien de dmo.

---

  Services utiliss (% gratuits)

| Service | Utilisation | Lien |
|---------|-----------|------|
| Vercel | Frontend (React/Vite) | https://vercel.com |
| Render.com | Backend (Go API) | https://render.com |
| Supabase | PostgreSQL manag | https://supabase.com |
| Redis Cloud | Cache Redis | https://app.redislabs.com |
| GitHub | Dpt + CI/CD | https://github.com |

---

  Prrequis

- [ ] Un compte GitHub (gratuit)
- [ ] Un dpt GitHub avec le code OpenRisk
- [ ] Accounts crs sur : Vercel, Render.com, Supabase, Redis Cloud

---

  Étape  : Prparer la base de donnes (Supabase)

 . Crer un compte Supabase
. Allez sur https://supabase.com
. Connectez-vous avec GitHub
. Crez un nouveau projet :
   - Nom : openrisk-demo
   - Rgion : Choisissez la plus proche
   - Mot de passe BD : Notez-le

 . Rcuprer les informations de connexion
. Allez dans Settings → Database
. Copiez la Connection string format PostgreSQL :
   
   postgresql://postgres:[PASSWORD]@[HOST]:/postgres
   

---

  Étape  : Dployer le Backend (Render.com)

 . Prparer le Dockerfile du backend

Le Dockerfile doit être adapt pour Render. Crez le fichier :

backend/Dockerfile.render
dockerfile
 Build stage
FROM golang:..-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

 Build the app
RUN CGO_ENABLED= GOOS=linux go build -o server ./cmd/server/main.go

 Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/server .
COPY --from=builder /app/migrations ./migrations

EXPOSE 

CMD ["./server"]


 . Connecter à Render.com

. Allez sur https://render.com
. Crez un nouveau Web Service
. Connectez votre dpt GitHub
. Configuration :
   - Name : openrisk-api
   - Environment : Docker
   - Region : Frankfurt (ou proche de vous)
   - Build Command : docker build -f backend/Dockerfile.render -t openrisk .
   - Start Command : ./server

 . Configurer les variables d'environnement sur Render

Allez dans Environment et ajoutez :

env
DATABASE_URL=postgresql://postgres:[PASSWORD]@[SUPABASE_HOST]:/postgres
JWT_SECRET=your-super-secret-key-min--chars-here-do-not-use-this
REDIS_URL=redis://default:[REDIS_PASSWORD]@[REDIS_HOST]:
PORT=
ENVIRONMENT=production
CORS_ORIGINS=https://your-frontend-domain.vercel.app
API_BASE_URL=https://openrisk-api.onrender.com
LOG_LEVEL=info


 . Dployer
- Cliquez sur Create Web Service
- Attendre - minutes
- Vous obtenez une URL : https://openrisk-api.onrender.com

---

  Étape  : Configurer Redis (Redis Cloud)

 . Crer une instance Redis gratuite

. Allez sur https://app.redislabs.com
. New Subscription → Free → Continue
. Configuration :
   - Cloud : AWS
   - Region : Frankfurt (ou proche)
   - Database : openrisk-cache

 . Rcuprer l'URL
. Cliquez sur votre base de donnes
. Dans Connectivity → Public endpoint :
   
   redis-.c.eu-west--.ec.cloud.redislabs.com:
   
. Copiez Default user password

---

  Étape  : Adapter le Frontend pour Vercel

 . Crer .env.production

frontend/.env.production
env
VITE_API_URL=https://openrisk-api.onrender.com
VITE_ENV=production


 . Mise à jour du vite.config.ts

typescript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': {
        target: process.env.VITE_API_URL || 'http://localhost:',
        changeOrigin: true,
        rewrite: (path) => path
      }
    }
  },
  build: {
    outDir: 'dist',
    sourcemap: false
  }
})


 . Mettre à jour l'appel API dans le code React

Vrifiez que votre client API utilise la variable d'environnement :

frontend/src/lib/api.ts (ou quivalent)
typescript
const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:'

const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json'
  }
})


---

  Étape  : Dployer le Frontend (Vercel)

 . Connecter Vercel à GitHub

. Allez sur https://vercel.com
. Import Project → Slectionnez votre dpt OpenRisk
. Configuration :
   - Root Directory : frontend
   - Framework Preset : Vite
   - Build Command : npm run build
   - Output Directory : dist

 . Ajouter les variables d'environnement

Dans Environment Variables :
env
VITE_API_URL=https://openrisk-api.onrender.com


 . Dployer
- Cliquez sur Deploy
- Attendre - minutes
- Vous obtenez : https://openrisk-xxxx.vercel.app

---

  Rsum des URLs

Aprs dploiement, vous aurez :

| Service | URL |
|---------|-----|
| Frontend | https://openrisk-xxxx.vercel.app |
| API | https://openrisk-api.onrender.com |
| API Docs | https://openrisk-api.onrender.com/swagger |

---

  Tester la dmo

. Ouvrez https://openrisk-xxxx.vercel.app
. Connectez-vous avec :
   - Email : admin@openrisk.local
   - Password : admin

---

  Limitations gratuits à connatre

| Service | Limitation |
|---------|-----------|
| Vercel | GB bande passante/mois, builds illimits |
| Render.com | Puts to sleep aprs  min inactivit (free tier) |
| Supabase |  MB stockage,  GB transfert donnes/mois |
| Redis Cloud |  MB RAM |

---

  Optimisations recommandes

 Pour Render.com (viter le sleep)
Ajouter un cron job gratuit pour faire un ping toutes les  minutes :
bash
 Service de ping externe
- https://uptimerobot.com (plan gratuit)
- https://cron-job.org


 Pour la DB Supabase
- Limiter les logs aux erreurs seulement
- Archiver les risques historiques aprs  jours
- Configurer le vacuum automatique

---

  Configuration CORS

Le backend doit autoriser votre frontend :

backend/cmd/server/main.go (vrifier la config CORS)
go
app.Use(cors.New(cors.Config{
    AllowOrigins: "https://openrisk-xxxx.vercel.app, http://localhost:",
    AllowMethods: "GET,POST,PUT,DELETE,PATCH,OPTIONS",
    AllowHeaders: "Content-Type,Authorization",
}))


---

  Dpannage

 Le frontend ne peut pas appeler l'API
-  Vrifiez VITE_API_URL dans Vercel
-  Vrifiez CORS_ORIGINS dans Render
-  Testez manuellement : curl https://openrisk-api.onrender.com/api/health

 Render sleep mode
-  Utilisez un service de monitoring gratuit pour viter le sleep
-  Ou passez à un plan payant ($/mois minimum)

 Base de donnes pleine ( MB Supabase)
-  Nettoyez les risques archivs
-  Upgradez vers un plan payant
-  Utilisez Railway.app pour PostgreSQL illimit (plan gratuit)

---

  Amliorations futures

Quand vous voudrez passer en production (avec plus de ressources) :

. Backend : Render.com → Heroku, Railway, ou VPS
. Frontend : Vercel → Netlify ou S + CloudFront
. DB : Supabase → AWS RDS, DigitalOcean, ou Azure
. Redis : Redis Cloud → Heroku Redis ou DigitalOcean Managed

---

Bon dploiement ! 
