  Guide de d√ploiement gratuit - OpenRisk

Ce guide vous explique comment d√ployer OpenRisk totalement gratuitement pour obtenir un lien de d√mo.

---

  Services utilis√s (% gratuits)

| Service | Utilisation | Lien |
|---------|-----------|------|
| Vercel | Frontend (React/Vite) | https://vercel.com |
| Render.com | Backend (Go API) | https://render.com |
| Supabase | PostgreSQL manag√ | https://supabase.com |
| Redis Cloud | Cache Redis | https://app.redislabs.com |
| GitHub | D√p√t + CI/CD | https://github.com |

---

  Pr√requis

- [ ] Un compte GitHub (gratuit)
- [ ] Un d√p√t GitHub avec le code OpenRisk
- [ ] Accounts cr√√s sur : Vercel, Render.com, Supabase, Redis Cloud

---

  √âtape  : Pr√parer la base de donn√es (Supabase)

 . Cr√er un compte Supabase
. Allez sur https://supabase.com
. Connectez-vous avec GitHub
. Cr√ez un nouveau projet :
   - Nom : openrisk-demo
   - R√gion : Choisissez la plus proche
   - Mot de passe BD : Notez-le

 . R√cup√rer les informations de connexion
. Allez dans Settings ‚Üí Database
. Copiez la Connection string format PostgreSQL :
   
   postgresql://postgres:[PASSWORD]@[HOST]:/postgres
   

---

  √âtape  : D√ployer le Backend (Render.com)

 . Pr√parer le Dockerfile du backend

Le Dockerfile doit √™tre adapt√ pour Render. Cr√ez le fichier :

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


 . Connecter √† Render.com

. Allez sur https://render.com
. Cr√ez un nouveau Web Service
. Connectez votre d√p√t GitHub
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


 . D√ployer
- Cliquez sur Create Web Service
- Attendre - minutes
- Vous obtenez une URL : https://openrisk-api.onrender.com

---

  √âtape  : Configurer Redis (Redis Cloud)

 . Cr√er une instance Redis gratuite

. Allez sur https://app.redislabs.com
. New Subscription ‚Üí Free ‚Üí Continue
. Configuration :
   - Cloud : AWS
   - Region : Frankfurt (ou proche)
   - Database : openrisk-cache

 . R√cup√rer l'URL
. Cliquez sur votre base de donn√es
. Dans Connectivity ‚Üí Public endpoint :
   
   redis-.c.eu-west--.ec.cloud.redislabs.com:
   
. Copiez Default user password

---

  √âtape  : Adapter le Frontend pour Vercel

 . Cr√er .env.production

frontend/.env.production
env
VITE_API_URL=https://openrisk-api.onrender.com
VITE_ENV=production


 . Mise √† jour du vite.config.ts

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


 . Mettre √† jour l'appel API dans le code React

V√rifiez que votre client API utilise la variable d'environnement :

frontend/src/lib/api.ts (ou √quivalent)
typescript
const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:'

const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json'
  }
})


---

  √âtape  : D√ployer le Frontend (Vercel)

 . Connecter Vercel √† GitHub

. Allez sur https://vercel.com
. Import Project ‚Üí S√lectionnez votre d√p√t OpenRisk
. Configuration :
   - Root Directory : frontend
   - Framework Preset : Vite
   - Build Command : npm run build
   - Output Directory : dist

 . Ajouter les variables d'environnement

Dans Environment Variables :
env
VITE_API_URL=https://openrisk-api.onrender.com


 . D√ployer
- Cliquez sur Deploy
- Attendre - minutes
- Vous obtenez : https://openrisk-xxxx.vercel.app

---

  R√sum√ des URLs

Apr√s d√ploiement, vous aurez :

| Service | URL |
|---------|-----|
| Frontend | https://openrisk-xxxx.vercel.app |
| API | https://openrisk-api.onrender.com |
| API Docs | https://openrisk-api.onrender.com/swagger |

---

 üß™ Tester la d√mo

. Ouvrez https://openrisk-xxxx.vercel.app
. Connectez-vous avec :
   - Email : admin@openrisk.local
   - Password : admin

---

  Limitations gratuits √† conna√tre

| Service | Limitation |
|---------|-----------|
| Vercel | GB bande passante/mois, builds illimit√s |
| Render.com | Puts to sleep apr√s  min inactivit√ (free tier) |
| Supabase |  MB stockage,  GB transfert donn√es/mois |
| Redis Cloud |  MB RAM |

---

  Optimisations recommand√es

 Pour Render.com (√viter le sleep)
Ajouter un cron job gratuit pour faire un ping toutes les  minutes :
bash
 Service de ping externe
- https://uptimerobot.com (plan gratuit)
- https://cron-job.org


 Pour la DB Supabase
- Limiter les logs aux erreurs seulement
- Archiver les risques historiques apr√s  jours
- Configurer le vacuum automatique

---

  Configuration CORS

Le backend doit autoriser votre frontend :

backend/cmd/server/main.go (v√rifier la config CORS)
go
app.Use(cors.New(cors.Config{
    AllowOrigins: "https://openrisk-xxxx.vercel.app, http://localhost:",
    AllowMethods: "GET,POST,PUT,DELETE,PATCH,OPTIONS",
    AllowHeaders: "Content-Type,Authorization",
}))


---

 üìû D√pannage

 Le frontend ne peut pas appeler l'API
-  V√rifiez VITE_API_URL dans Vercel
-  V√rifiez CORS_ORIGINS dans Render
-  Testez manuellement : curl https://openrisk-api.onrender.com/api/health

 Render sleep mode
-  Utilisez un service de monitoring gratuit pour √viter le sleep
-  Ou passez √† un plan payant ($/mois minimum)

 Base de donn√es pleine ( MB Supabase)
-  Nettoyez les risques archiv√s
-  Upgradez vers un plan payant
-  Utilisez Railway.app pour PostgreSQL illimit√ (plan gratuit)

---

  Am√liorations futures

Quand vous voudrez passer en production (avec plus de ressources) :

. Backend : Render.com ‚Üí Heroku, Railway, ou VPS
. Frontend : Vercel ‚Üí Netlify ou S + CloudFront
. DB : Supabase ‚Üí AWS RDS, DigitalOcean, ou Azure
. Redis : Redis Cloud ‚Üí Heroku Redis ou DigitalOcean Managed

---

Bon d√ploiement ! 
