# 🔌 Integration Guide - Connexion Frontend/Backend

Ce guide couvre les étapes d'intégration pour que le frontend communique correctement avec le backend déployé.

---

## 📋 Configuration Frontend

### 1. Variables d'environnement

**`frontend/.env.production`**
```env
# Production API URL
VITE_API_URL=https://openrisk-api.onrender.com

# Environment
VITE_ENV=production
```

**`frontend/.env.development`** (pour tests locaux)
```env
VITE_API_URL=http://localhost:8080
VITE_ENV=development
```

### 2. Configuration Vite (vite.config.ts)

```typescript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  
  server: {
    // Proxy API calls during dev
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        secure: false,
      }
    },
    port: 5173,
    host: 'localhost'
  },
  
  build: {
    outDir: 'dist',
    sourcemap: false, // Disable in production
    minify: 'terser'
  }
})
```

### 3. Client API (axios ou fetch)

**`frontend/src/lib/api.ts`** (exemple avec axios)

```typescript
import axios from 'axios'

// Get API URL from environment variable
const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

// Create axios instance
const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  }
})

// Add token to requests
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// Handle responses
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      // Redirect to login
      localStorage.removeItem('token')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

export default api
```

### 4. Exemple d'utilisation

```typescript
import api from '@/lib/api'

// Get risks
const getRisks = async () => {
  try {
    const response = await api.get('/api/v1/risks')
    return response.data
  } catch (error) {
    console.error('Failed to fetch risks:', error)
    throw error
  }
}

// Create risk
const createRisk = async (riskData) => {
  try {
    const response = await api.post('/api/v1/risks', riskData)
    return response.data
  } catch (error) {
    console.error('Failed to create risk:', error)
    throw error
  }
}
```

---

## 🔒 Configuration Backend (CORS)

### Backend Configuration (`cmd/server/main.go`)

Vérifiez que le CORS est configuré correctement :

```go
import "github.com/gofiber/fiber/v2/middleware/cors"

func main() {
  app := fiber.New()
  
  // CORS Middleware
  app.Use(cors.New(cors.Config{
    AllowOrigins: os.Getenv("CORS_ORIGINS"), // Set via env variable
    AllowMethods: "GET,POST,PUT,DELETE,PATCH,OPTIONS",
    AllowHeaders: "Content-Type,Authorization,X-Requested-With",
    AllowCredentials: true,
    MaxAge: 3600,
  }))
  
  // ... rest of config
}
```

### Variables d'environnement Backend

**Pour développement local** (`.env`)
```env
CORS_ORIGINS=http://localhost:5173,http://localhost:3000
```

**Pour production** (Render.com)
```env
CORS_ORIGINS=https://openrisk-xxxx.vercel.app
```

---

## 🧪 Étapes de test d'intégration

### Test 1: Ping l'API

```bash
# Test health endpoint
curl -X GET https://openrisk-api.onrender.com/api/health

# Expected response:
# {"status":"OK"}
```

### Test 2: Test CORS depuis le frontend

Ouvrez la console DevTools du browser et exécutez :

```javascript
// Test de la requête
fetch('https://openrisk-api.onrender.com/api/health')
  .then(r => r.json())
  .then(data => console.log('✅ Success:', data))
  .catch(err => console.error('❌ Error:', err))
```

### Test 3: Test d'authentification

```javascript
// Login
const loginResponse = await fetch('https://openrisk-api.onrender.com/api/auth/login', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    email: process.env.OPENRISK_EMAIL,
    password: process.env.OPENRISK_PASSWORD, // never hard-code it
  })
})

const { token } = await loginResponse.json()
console.log('Token:', token)

// Use token for protected requests
const risksResponse = await fetch('https://openrisk-api.onrender.com/api/risks', {
  headers: { 'Authorization': `Bearer ${token}` }
})

const risks = await risksResponse.json()
console.log('Risks:', risks)
```

### Test 4: Vérifier depuis la console du browser

Une fois le frontend chargé, testez dans la console :

```javascript
// Check environment
console.log('API Base URL:', import.meta.env.VITE_API_URL)

// Try a simple API call
import api from '@/lib/api'
api.get('/api/health')
  .then(r => console.log('✅ API Health:', r.data))
  .catch(e => console.error('❌ CORS or API Error:', e))
```

---

## ❌ Dépannage des erreurs courantes

### 1. CORS Error: "No 'Access-Control-Allow-Origin'"

**Cause**: `CORS_ORIGINS` ne contient pas votre Vercel URL

**Solution**:
```bash
# Dans Render.com, mettre à jour:
CORS_ORIGINS=https://openrisk-xxxx.vercel.app

# Puis redéployer le service
```

### 2. 401 Unauthorized - "Token invalid or expired"

**Cause**: JWT_SECRET ne correspond pas entre services

**Solution**:
```bash
# Vérifier que JWT_SECRET est identique sur Render
# Redéployer si changé
```

### 3. API Endpoint returns 404

**Cause**: Mauvaise API URL ou endpoint invalide

**Solution**:
```bash
# Vérifier l'API URL dans Vercel env:
VITE_API_URL=https://openrisk-api.onrender.com

# Tester l'endpoint:
curl https://openrisk-api.onrender.com/api/risks
```

### 4. Network Error - "Cannot reach API"

**Cause**: 
- Backend service is sleeping (Render free tier)
- API URL est incorrecte
- Connexion réseau

**Solution**:
```bash
# 1. Vérifier que Render service est "Live":
curl https://openrisk-api.onrender.com/api/health

# 2. Si dormant, attendre le réveil (30-60 sec)

# 3. Configurer monitoring gratuit pour éviter le sleep:
# https://uptimerobot.com
```

### 5. Blank page / Frontend ne charge pas

**Cause**: Erreur JavaScript ou build

**Solution**:
```bash
# Vérifier les logs Vercel
# Vérifier la console DevTools
# Vérifier Network tab

# Rebuild sur Vercel:
# Dans dashboard → Deployments → Redeploy
```

---

## 🔍 Diagnostic avancé

### Vérifier les logs backend (Render)

1. Allez sur https://render.com
2. Cliquez sur votre service `openrisk-api`
3. Onglet **Logs** pour voir les erreurs
4. Cherchez les patterns :
   - "CORS"
   - "Database"
   - "ERROR"
   - "Connection refused"

### Vérifier les logs frontend (Vercel)

1. Allez sur https://vercel.com
2. Cliquez sur votre projet
3. Onglet **Deployments**
4. Cliquez sur le dernier déploiement
5. Onglet **Logs** → **Build** ou **Runtime**

### Network Debugging (Browser DevTools)

```
1. F12 → Network tab
2. Testez une action (login, fetch data)
3. Cherchez votre API call
4. Vérifiez:
   - Status (200 = ok, 4xx = client error, 5xx = server error)
   - Response headers (Access-Control-Allow-Origin)
   - Response body
   - Timings
```

---

## 📊 Checklist de configuration

- [ ] `frontend/.env.production` contient `VITE_API_URL` correct
- [ ] `VITE_API_URL` est l'URL Render sans trailing slash
- [ ] Backend a `CORS_ORIGINS` contenant l'URL Vercel exact
- [ ] JWT_SECRET est identique sur backend
- [ ] Database connection fonctionne (vérifier logs Render)
- [ ] Redis connection fonctionne
- [ ] Frontend peut atteindre `/api/health`
- [ ] Frontend peut authenticater (login)
- [ ] Frontend peut fetch des données (risks, etc.)
- [ ] Pas d'erreurs console dans browser
- [ ] Pas d'erreurs CORS
- [ ] Pas d'erreurs 401/403 (sauf après logout)

---

## 🚀 Prochaines étapes

Une fois l'intégration validée :

1. Testez toutes les features (create, update, delete)
2. Testez la pagination et le filtrage
3. Testez l'export PDF
4. Testez les notifications
5. Testez les permissions et les rôles

Puis partagez votre démo ! 🎉
