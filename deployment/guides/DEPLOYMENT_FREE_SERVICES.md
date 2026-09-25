# 🚀 Guide de déploiement gratuit - OpenRisk

Ce guide vous explique comment déployer OpenRisk totalement gratuitement pour obtenir un lien de démo.

---

## 📋 Services utilisés (100% gratuits)

| Service | Utilisation | Lien |
|---------|-----------|------|
| **Vercel** | Frontend (React/Vite) | https://vercel.com |
| **Render.com** | Backend (Go API) | https://render.com |
| **Supabase** | PostgreSQL managé | https://supabase.com |
| **Redis Cloud** | Cache Redis | https://app.redislabs.com |
| **GitHub** | Dépôt + CI/CD | https://github.com |

---

## ✅ Prérequis

- [ ] Un compte GitHub (gratuit)
- [ ] Un dépôt GitHub avec le code OpenRisk
- [ ] Accounts créés sur : Vercel, Render.com, Supabase, Redis Cloud

---

## 🎯 Étape 1 : Préparer la base de données (Supabase)

### 1.1 Créer un compte Supabase
1. Allez sur https://supabase.com
2. Connectez-vous avec GitHub
3. Créez un nouveau projet :
   - **Nom** : `openrisk-demo`
   - **Région** : Choisissez la plus proche
   - **Mot de passe BD** : Notez-le

### 1.2 Récupérer les informations de connexion
1. Allez dans **Settings** → **Database**
2. Copiez la **Connection string** format PostgreSQL :
   ```
   postgresql://postgres:[PASSWORD]@[HOST]:5432/postgres
   ```

---

## 🎯 Étape 2 : Déployer le Backend (Render.com)

### 2.1 Préparer le Dockerfile du backend

Le Dockerfile doit être adapté pour Render. Créez le fichier :

**`backend/Dockerfile.render`**
```dockerfile
# Build stage
FROM golang:1.25.4-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the app
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server/main.go

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/server .
COPY --from=builder /app/migrations ./migrations

EXPOSE 8080

CMD ["./server"]
```

### 2.2 Connecter à Render.com

1. Allez sur https://render.com
2. Créez un nouveau **Web Service**
3. Connectez votre dépôt GitHub
4. Configuration :
   - **Name** : `openrisk-api`
   - **Environment** : Docker
   - **Region** : Frankfurt (ou proche de vous)
   - **Build Command** : `docker build -f backend/Dockerfile.render -t openrisk .`
   - **Start Command** : `./server`

### 2.3 Configurer les variables d'environnement sur Render

Allez dans **Environment** et ajoutez :

```env
DATABASE_URL=postgresql://postgres:[PASSWORD]@[SUPABASE_HOST]:5432/postgres
RSA_PRIVATE_KEY=<PEM private key, see scripts/generate_rsa_keys.sh>
RSA_PUBLIC_KEY=<PEM public key>
REDIS_URL=redis://default:[REDIS_PASSWORD]@[REDIS_HOST]:19999
PORT=8080
ENVIRONMENT=production
CORS_ORIGINS=https://your-frontend-domain.vercel.app
API_BASE_URL=https://openrisk-api.onrender.com
LOG_LEVEL=info
```

### 2.4 Déployer
- Cliquez sur **Create Web Service**
- Attendre 3-5 minutes
- Vous obtenez une URL : `https://openrisk-api.onrender.com`

---

## 🎯 Étape 3 : Configurer Redis (Redis Cloud)

### 3.1 Créer une instance Redis gratuite

1. Allez sur https://app.redislabs.com
2. **New Subscription** → **Free** → **Continue**
3. Configuration :
   - **Cloud** : AWS
   - **Region** : Frankfurt (ou proche)
   - **Database** : `openrisk-cache`

### 3.2 Récupérer l'URL
1. Cliquez sur votre base de données
2. Dans **Connectivity** → **Public endpoint** :
   ```
   redis-12345.c245.eu-west-1-2.ec2.cloud.redislabs.com:19999
   ```
3. Copiez **Default user password**

---

## 🎯 Étape 4 : Adapter le Frontend pour Vercel

### 4.1 Créer `.env.production`

**`frontend/.env.production`**
```env
VITE_API_URL=https://openrisk-api.onrender.com
VITE_ENV=production
```

### 4.2 Mise à jour du `vite.config.ts`

```typescript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': {
        target: process.env.VITE_API_URL || 'http://localhost:8080',
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
```

### 4.3 Mettre à jour l'appel API dans le code React

Vérifiez que votre client API utilise la variable d'environnement :

**`frontend/src/lib/api.ts`** (ou équivalent)
```typescript
const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json'
  }
})
```

---

## 🎯 Étape 5 : Déployer le Frontend (Vercel)

### 5.1 Connecter Vercel à GitHub

1. Allez sur https://vercel.com
2. **Import Project** → Sélectionnez votre dépôt OpenRisk
3. Configuration :
   - **Root Directory** : `frontend`
   - **Framework Preset** : Vite
   - **Build Command** : `npm run build`
   - **Output Directory** : `dist`

### 5.2 Ajouter les variables d'environnement

Dans **Environment Variables** :
```env
VITE_API_URL=https://openrisk-api.onrender.com
```

### 5.3 Déployer
- Cliquez sur **Deploy**
- Attendre 2-3 minutes
- Vous obtenez : `https://openrisk-xxxx.vercel.app`

---

## 🔗 Résumé des URLs

Après déploiement, vous aurez :

| Service | URL |
|---------|-----|
| **Frontend** | `https://openrisk-xxxx.vercel.app` |
| **API** | `https://openrisk-api.onrender.com` |
| **API Docs** | `https://openrisk-api.onrender.com/swagger` |

---

## 🧪 Tester la démo

1. Ouvrez https://openrisk-xxxx.vercel.app
2. Connectez-vous avec :
   - **Email** : `admin@opendefender.io`
   - **Mot de passe** : le mot de passe défini dans `INITIAL_ADMIN_PASSWORD`, ou celui généré au premier démarrage dans `/app/secrets/initial_admin_password`. Il n'existe aucun mot de passe par défaut.

---

## ⚠️ Limitations gratuits à connaître

| Service | Limitation |
|---------|-----------|
| Vercel | 100GB bande passante/mois, builds illimités |
| Render.com | Puts to sleep après 15 min inactivité (free tier) |
| Supabase | 500 MB stockage, 2 GB transfert données/mois |
| Redis Cloud | 30 MB RAM |

---

## 🚀 Optimisations recommandées

### Pour Render.com (éviter le sleep)
Ajouter un cron job gratuit pour faire un ping toutes les 14 minutes :
```bash
# Service de ping externe
- https://uptimerobot.com (plan gratuit)
- https://cron-job.org
```

### Pour la DB Supabase
- Limiter les logs aux erreurs seulement
- Archiver les risques historiques après 90 jours
- Configurer le vacuum automatique

---

## 🔧 Configuration CORS

Le backend doit autoriser votre frontend :

**`backend/cmd/server/main.go`** (vérifier la config CORS)
```go
app.Use(cors.New(cors.Config{
    AllowOrigins: "https://openrisk-xxxx.vercel.app, http://localhost:5173",
    AllowMethods: "GET,POST,PUT,DELETE,PATCH,OPTIONS",
    AllowHeaders: "Content-Type,Authorization",
}))
```

---

## 📞 Dépannage

### Le frontend ne peut pas appeler l'API
- ✅ Vérifiez `VITE_API_URL` dans Vercel
- ✅ Vérifiez `CORS_ORIGINS` dans Render
- ✅ Testez manuellement : `curl https://openrisk-api.onrender.com/api/health`

### Render sleep mode
- ✅ Utilisez un service de monitoring gratuit pour éviter le sleep
- ✅ Ou passez à un plan payant ($7/mois minimum)

### Base de données pleine (500 MB Supabase)
- ✅ Nettoyez les risques archivés
- ✅ Upgradez vers un plan payant
- ✅ Utilisez Railway.app pour PostgreSQL illimité (plan gratuit)

---

## ✨ Améliorations futures

Quand vous voudrez passer en production (avec plus de ressources) :

1. **Backend** : Render.com → Heroku, Railway, ou VPS
2. **Frontend** : Vercel → Netlify ou S3 + CloudFront
3. **DB** : Supabase → AWS RDS, DigitalOcean, ou Azure
4. **Redis** : Redis Cloud → Heroku Redis ou DigitalOcean Managed

---

**Bon déploiement ! 🎉**
