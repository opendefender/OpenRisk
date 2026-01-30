 üîå Integration Guide - Connexion Frontend/Backend

Ce guide couvre les √tapes d'int√gration pour que le frontend communique correctement avec le backend d√ploy√.

---

  Configuration Frontend

 . Variables d'environnement

frontend/.env.production
env
 Production API URL
VITE_API_URL=https://openrisk-api.onrender.com

 Environment
VITE_ENV=production


frontend/.env.development (pour tests locaux)
env
VITE_API_URL=http://localhost:
VITE_ENV=development


 . Configuration Vite (vite.config.ts)

typescript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  
  server: {
    // Proxy API calls during dev
    proxy: {
      '/api': {
        target: 'http://localhost:',
        changeOrigin: true,
        secure: false,
      }
    },
    port: ,
    host: 'localhost'
  },
  
  build: {
    outDir: 'dist',
    sourcemap: false, // Disable in production
    minify: 'terser'
  }
})


 . Client API (axios ou fetch)

frontend/src/lib/api.ts (exemple avec axios)

typescript
import axios from 'axios'

// Get API URL from environment variable
const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:'

// Create axios instance
const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: ,
  headers: {
    'Content-Type': 'application/json',
  }
})

// Add token to requests
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = Bearer ${token}
  }
  return config
})

// Handle responses
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === ) {
      // Redirect to login
      localStorage.removeItem('token')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

export default api


 . Exemple d'utilisation

typescript
import api from '@/lib/api'

// Get risks
const getRisks = async () => {
  try {
    const response = await api.get('/api/v/risks')
    return response.data
  } catch (error) {
    console.error('Failed to fetch risks:', error)
    throw error
  }
}

// Create risk
const createRisk = async (riskData) => {
  try {
    const response = await api.post('/api/v/risks', riskData)
    return response.data
  } catch (error) {
    console.error('Failed to create risk:', error)
    throw error
  }
}


---

 üîí Configuration Backend (CORS)

 Backend Configuration (cmd/server/main.go)

V√rifiez que le CORS est configur√ correctement :

go
import "github.com/gofiber/fiber/v/middleware/cors"

func main() {
  app := fiber.New()
  
  // CORS Middleware
  app.Use(cors.New(cors.Config{
    AllowOrigins: os.Getenv("CORS_ORIGINS"), // Set via env variable
    AllowMethods: "GET,POST,PUT,DELETE,PATCH,OPTIONS",
    AllowHeaders: "Content-Type,Authorization,X-Requested-With",
    AllowCredentials: true,
    MaxAge: ,
  }))
  
  // ... rest of config
}


 Variables d'environnement Backend

Pour d√veloppement local (.env)
env
CORS_ORIGINS=http://localhost:,http://localhost:


Pour production (Render.com)
env
CORS_ORIGINS=https://openrisk-xxxx.vercel.app


---

 üß™ √âtapes de test d'int√gration

 Test : Ping l'API

bash
 Test health endpoint
curl -X GET https://openrisk-api.onrender.com/api/health

 Expected response:
 {"status":"OK"}


 Test : Test CORS depuis le frontend

Ouvrez la console DevTools du browser et ex√cutez :

javascript
// Test de la requ√™te
fetch('https://openrisk-api.onrender.com/api/health')
  .then(r => r.json())
  .then(data => console.log(' Success:', data))
  .catch(err => console.error(' Error:', err))


 Test : Test d'authentification

javascript
// Login
const loginResponse = await fetch('https://openrisk-api.onrender.com/api/auth/login', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    email: 'admin@openrisk.local',
    password: 'admin'
  })
})

const { token } = await loginResponse.json()
console.log('Token:', token)

// Use token for protected requests
const risksResponse = await fetch('https://openrisk-api.onrender.com/api/risks', {
  headers: { 'Authorization': Bearer ${token} }
})

const risks = await risksResponse.json()
console.log('Risks:', risks)


 Test : V√rifier depuis la console du browser

Une fois le frontend charg√, testez dans la console :

javascript
// Check environment
console.log('API Base URL:', import.meta.env.VITE_API_URL)

// Try a simple API call
import api from '@/lib/api'
api.get('/api/health')
  .then(r => console.log(' API Health:', r.data))
  .catch(e => console.error(' CORS or API Error:', e))


---

  D√pannage des erreurs courantes

 . CORS Error: "No 'Access-Control-Allow-Origin'"

Cause: CORS_ORIGINS ne contient pas votre Vercel URL

Solution:
bash
 Dans Render.com, mettre √† jour:
CORS_ORIGINS=https://openrisk-xxxx.vercel.app

 Puis red√ployer le service


 .  Unauthorized - "Token invalid or expired"

Cause: JWT_SECRET ne correspond pas entre services

Solution:
bash
 V√rifier que JWT_SECRET est identique sur Render
 Red√ployer si chang√


 . API Endpoint returns 

Cause: Mauvaise API URL ou endpoint invalide

Solution:
bash
 V√rifier l'API URL dans Vercel env:
VITE_API_URL=https://openrisk-api.onrender.com

 Tester l'endpoint:
curl https://openrisk-api.onrender.com/api/risks


 . Network Error - "Cannot reach API"

Cause: 
- Backend service is sleeping (Render free tier)
- API URL est incorrecte
- Connexion r√seau

Solution:
bash
 . V√rifier que Render service est "Live":
curl https://openrisk-api.onrender.com/api/health

 . Si dormant, attendre le r√veil (- sec)

 . Configurer monitoring gratuit pour √viter le sleep:
 https://uptimerobot.com


 . Blank page / Frontend ne charge pas

Cause: Erreur JavaScript ou build

Solution:
bash
 V√rifier les logs Vercel
 V√rifier la console DevTools
 V√rifier Network tab

 Rebuild sur Vercel:
 Dans dashboard ‚Üí Deployments ‚Üí Redeploy


---

 üîç Diagnostic avanc√

 V√rifier les logs backend (Render)

. Allez sur https://render.com
. Cliquez sur votre service openrisk-api
. Onglet Logs pour voir les erreurs
. Cherchez les patterns :
   - "CORS"
   - "Database"
   - "ERROR"
   - "Connection refused"

 V√rifier les logs frontend (Vercel)

. Allez sur https://vercel.com
. Cliquez sur votre projet
. Onglet Deployments
. Cliquez sur le dernier d√ploiement
. Onglet Logs ‚Üí Build ou Runtime

 Network Debugging (Browser DevTools)


. F ‚Üí Network tab
. Testez une action (login, fetch data)
. Cherchez votre API call
. V√rifiez:
   - Status ( = ok, xx = client error, xx = server error)
   - Response headers (Access-Control-Allow-Origin)
   - Response body
   - Timings


---

  Checklist de configuration

- [ ] frontend/.env.production contient VITE_API_URL correct
- [ ] VITE_API_URL est l'URL Render sans trailing slash
- [ ] Backend a CORS_ORIGINS contenant l'URL Vercel exact
- [ ] JWT_SECRET est identique sur backend
- [ ] Database connection fonctionne (v√rifier logs Render)
- [ ] Redis connection fonctionne
- [ ] Frontend peut atteindre /api/health
- [ ] Frontend peut authenticater (login)
- [ ] Frontend peut fetch des donn√es (risks, etc.)
- [ ] Pas d'erreurs console dans browser
- [ ] Pas d'erreurs CORS
- [ ] Pas d'erreurs / (sauf apr√s logout)

---

  Prochaines √tapes

Une fois l'int√gration valid√e :

. Testez toutes les features (create, update, delete)
. Testez la pagination et le filtrage
. Testez l'export PDF
. Testez les notifications
. Testez les permissions et les r√les

Puis partagez votre d√mo ! 
