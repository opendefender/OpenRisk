  Configuration Files

Environment configuration templates for OpenRisk deployment.

  Files

 .env.production
Frontend environment variables for Vercel.

env
VITE_API_URL=https://openrisk-api.onrender.com
VITE_ENV=production


Location: frontend/.env.production (version controlled)

 .env.backend.example
Backend environment variables template for Render.com.

DO NOT commit actual secrets! This is a template only.

Setup Instructions:
. Copy .env.backend.example to a local file
. Fill in your actual values:
   - DATABASE_URL from Supabase
   - REDIS_URL from Redis Cloud
   - JWT_SECRET (generate with: openssl rand -base )
   - CORS_ORIGINS with your Vercel URL
. Add these variables directly in Render.com dashboard (Environment tab)

Required Variables:
- DATABASE_URL - PostgreSQL connection string
- REDIS_URL - Redis Cloud connection string
- JWT_SECRET - + character random secret
- CORS_ORIGINS - Your Vercel frontend URL
- API_BASE_URL - Your Render backend URL
- PORT - Usually 
- ENVIRONMENT - Set to production

---

  How to Generate JWT_SECRET

bash
 Generate a -character base secret
openssl rand -base 


Copy the output and use it as JWT_SECRET in Render.com environment variables.

---

  Render.com Setup

. Go to https://render.com
. Select your openrisk-api web service
. Go to Environment tab
. Add these variables from .env.backend.example:
   - DATABASE_URL
   - REDIS_URL
   - JWT_SECRET
   - CORS_ORIGINS
   - API_BASE_URL
   - LOG_LEVEL
   - ENVIRONMENT
   - PORT

. Click Save and Deploy

---

  Vercel Setup

. Go to https://vercel.com
. Select your openrisk project
. Go to Settings → Environment Variables
. Add from .env.production:
   - VITE_API_URL = https://openrisk-api.onrender.com

. Click Save and redeploy

---

  Security Notes

. Never commit .env files with secrets!
   - Only example files (.example) are version controlled
   - Real secrets go in service dashboards

. JWT_SECRET
   - Must be at least  characters
   - Generate random: openssl rand -base 
   - Keep it secret! Don't share

. Database URL
   - Contains your Supabase password
   - Keep it confidential
   - Only in Render.com (not in GitHub)

. CORS_ORIGINS
   - Must be your exact Vercel URL
   - Example: https://openrisk-xxxx.vercel.app
   - No trailing slash

---

  Environment Variables Reference

 Frontend (Vercel - .env.production)
env
VITE_API_URL=https://openrisk-api.onrender.com
VITE_ENV=production


 Backend (Render.com - Dashboard)
env
 Database (from Supabase)
DATABASE_URL=postgresql://postgres:PASSWORD@host.supabase.co:/postgres

 Cache (from Redis Cloud)
REDIS_URL=redis://default:PASSWORD@host.redislabs.com:

 Server
PORT=
ENVIRONMENT=production
LOG_LEVEL=info

 Security
JWT_SECRET=generated--char-random-string
JWT_EXPIRY=h

 API
CORS_ORIGINS=https://openrisk-xxxx.vercel.app
API_BASE_URL=https://openrisk-api.onrender.com

 Database Connection
DB_MAX_CONNECTIONS=
DB_CONNECTION_TIMEOUT=s


---

  Verification

After setting environment variables:

Test Frontend:
bash
curl https://openrisk-xxxx.vercel.app
 Should return HTML (login page)


Test Backend:
bash
curl https://openrisk-api.onrender.com/api/health
 Should return: {"status":"OK"}


Test API with Auth:
bash
 Get token
TOKEN=$(curl -X POST https://openrisk-api.onrender.com/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@openrisk.local","password":"admin"}' \
  | jq -r '.token')

 Use token
curl https://openrisk-api.onrender.com/api/risks \
  -H "Authorization: Bearer $TOKEN"


---

  Troubleshooting

API returns  (Unauthorized)
→ Check JWT_SECRET matches between frontend & backend

API returns CORS error
→ Check CORS_ORIGINS matches your exact Vercel URL

Database connection fails
→ Check DATABASE_URL in Render environment

Redis connection fails
→ Check REDIS_URL in Render environment

---

Need more help? See deployment/guides/INTEGRATION_GUIDE.md
