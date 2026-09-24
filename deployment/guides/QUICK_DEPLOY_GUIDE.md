# 🚀 Quick Start - Déploiement gratuit en 30 minutes

## Résumé rapide

Pour obtenir un lien de démo en 30 minutes avec **zéro frais**, voici les 4 étapes :

### 1️⃣ Base de données PostgreSQL (Supabase) - 5 min

```bash
1. Allez sur https://supabase.com
2. Sign up with GitHub
3. New Project → openrisk-demo
4. Récupérez: CONNECTION STRING (Settings → Database)
   Format: postgresql://postgres:PASSWORD@host.supabase.co:5432/postgres
```

### 2️⃣ Cache Redis (Redis Cloud) - 5 min

```bash
1. Allez sur https://app.redislabs.com
2. Sign up → Free tier
3. New Database → 30 MB
4. Récupérez: redis-endpoint:port et PASSWORD
   Format: redis://default:PASSWORD@host.redislabs.com:19999
```

### 3️⃣ Backend API (Render.com) - 10 min

```bash
1. Allez sur https://render.com
2. Sign up with GitHub → Connect repo OpenRisk
3. New Web Service:
   - Name: openrisk-api
   - Environment: Docker
   - Build Command: docker build -f Dockerfile.render -t openrisk .
   
4. Environment Variables:
   DATABASE_URL=postgresql://postgres:PASSWORD@...
   REDIS_URL=redis://default:PASSWORD@...
   JWT_SECRET=generez-une-clé-de-32-chars
   CORS_ORIGINS=https://openrisk-xxxx.vercel.app (ajouter après Vercel)
   API_BASE_URL=https://openrisk-api.onrender.com
   
5. Deploy → Attendre 3-5 minutes
   URL résultante: https://openrisk-api.onrender.com
```

### 4️⃣ Frontend (Vercel) - 10 min

```bash
1. Allez sur https://vercel.com
2. Sign up with GitHub → Import Project
3. Configuration:
   - Select OpenRisk repository
   - Root Directory: frontend
   - Framework: Vite
   - Build Command: npm run build
   
4. Environment Variable:
   VITE_API_URL=https://openrisk-api.onrender.com
   
5. Deploy → Attendre 2-3 minutes
   URL résultante: https://openrisk-xxxx.vercel.app
```

---

## ✅ Vérification finale

1. **Testez l'API**:
   ```bash
   curl https://openrisk-api.onrender.com/api/health
   ```

2. **Testez le frontend**:
   ```
   https://openrisk-xxxx.vercel.app
   Email: admin@opendefender.io
   Mot de passe: INITIAL_ADMIN_PASSWORD, ou /app/secrets/initial_admin_password
   ```

3. **Docs API Swagger**:
   ```
   https://openrisk-api.onrender.com/swagger
   ```

---

## 🔑 Premier accès

Il n'existe aucun identifiant par défaut. Le premier démarrage crée
`admin@opendefender.io` avec le mot de passe défini dans `INITIAL_ADMIN_PASSWORD`, ou celui généré au premier démarrage dans `/app/secrets/initial_admin_password` (mode 0600, jamais écrit
dans les logs). Changez-le après la première connexion, puis supprimez ce fichier.

---

## 📊 Stack de déploiement

```
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
      ┌────────┴────────┐
      ▼                  ▼
┌──────────────┐  ┌──────────────────┐
│   Supabase   │  │   Redis Cloud    │
│  PostgreSQL  │  │   Cache (30 MB)  │
│  (500 MB)    │  └──────────────────┘
└──────────────┘
```

---

## ⚠️ Limites gratuites à connaître

| Service | Limite | Contournement |
|---------|--------|---------------|
| Render.com | Sleep après 15 min inactivité | Utilisez uptimerobot.com (gratuit) pour ping |
| Vercel | 100 GB/mois bande passante | Optimisez images, utilisez CDN |
| Supabase | 500 MB DB + 2 GB transfert | Archivez les anciens risques |
| Redis Cloud | 30 MB RAM | Nettoyez le cache régulièrement |

---

## 🔧 Commandes utiles

### Générer un JWT_SECRET robuste
```bash
openssl rand -base64 32
```

### Tester la connexion DB Supabase
```bash
psql "postgresql://postgres:PASSWORD@host.supabase.co:5432/postgres" -c "SELECT 1"
```

### Tester Redis
```bash
redis-cli -h host.redislabs.com -p 19999 -a PASSWORD ping
```

---

## 🚨 Dépannage rapide

### ❌ "CORS error - frontend cannot reach API"
```
→ Dans Render, vérifier CORS_ORIGINS contient votre Vercel URL
→ Exemple: CORS_ORIGINS=https://openrisk-xxxx.vercel.app
```

### ❌ "Database connection error"
```
→ Vérifier DATABASE_URL dans Render env
→ Tester: psql "postgresql://..."
```

### ❌ "Render service goes to sleep"
```
→ Ajouter monitoring gratuit: https://uptimerobot.com
→ Ping toutes les 14 minutes: https://openrisk-api.onrender.com/api/health
```

### ❌ "Cannot login - admin user not created"
```
→ Vérifier que les migrations DB ont roulé
→ Dans Render logs, chercher "Database: Running Auto-Migrations"
```

---

## 📚 Documentation complète

Pour les détails complets, consultez: **DEPLOYMENT_FREE_SERVICES.md**

---

## 💰 Coût total

🎉 **$0.00/mois**

Tous les services utilisés ont des plans gratuits généreux !

---

## 🎯 Prochaines étapes après le déploiement

1. ✅ Créez des comptes utilisateur
2. ✅ Ajoutez des risques de test
3. ✅ Testez la création de mitigations
4. ✅ Validez les dashboards
5. ✅ Partagez le lien de démo : `https://openrisk-xxxx.vercel.app`

---

**Bon déploiement ! 🚀**
