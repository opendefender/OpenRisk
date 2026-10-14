# 🏗️ Architecture de déploiement OpenRisk

## Diagramme global

```
                        🌍 INTERNET 🌍
                        
    User Browser          Mobile App         API Clients
           │                  │                    │
           └──────────────────┼────────────────────┘
                              │
                        HTTPS (TLS/SSL)
                              │
      ╔═════════════════════════════════════════╗
      ║          🟦 VERCEL CDN GLOBAL           ║
      ║   https://openrisk-xxxx.vercel.app     ║
      ║                                         ║
      ║  Frontend (React + Vite + TailwindCSS) ║
      ║  ✅ Auto-deploy from GitHub            ║
      ║  ✅ Global CDN                         ║
      ║  ✅ 100GB/mois bandwidth               ║
      ║  ✅ HTTPS automatic                    ║
      ╚═════════════════════════════════════════╝
                              │
                              │ HTTPS API Calls
                              │ (JSON REST)
                              ▼
      ╔═════════════════════════════════════════╗
      ║      🟩 RENDER.COM - BACKEND           ║
      ║  https://openrisk-api.onrender.com     ║
      ║                                         ║
      ║  Go 1.26 + Fiber API Server          ║
      ║  ✅ Docker container                   ║
      ║  ✅ Auto-deploy from GitHub            ║
      ║  ✅ Free tier with 15min sleep         ║
      ║  ✅ HTTPS automatic                    ║
      ╚═════════════════════════════════════════╝
                              │
                ┌─────────────┼─────────────┐
                │             │             │
           TCP/IP         TCP/IP         TCP/IP
                │             │             │
                ▼             ▼             ▼
    ╔═══════════════════╗ ╔═════════════╗ ╔══════════════╗
    ║   🟪 SUPABASE     ║ ║ 🔴 REDIS    ║ ║ 📝 LOGS      ║
    ║                 ║ ║ CLOUD       ║ ║              ║
    ║  PostgreSQL DB  ║ ║             ║ ║ Server Logs  ║
    ║  500 MB Storage ║ ║ 30 MB Cache ║ ║ Request Logs ║
    ║  2GB trans/mo   ║ ║ Sessions    ║ ║              ║
    ║                 ║ ║ Caching     ║ ║ Render/Vercel║
    ╚═══════════════════╝ ╚═════════════╝ ╚══════════════╝
```

## Architecture détaillée par composant

### 1️⃣ Frontend Layer (Vercel)

```
                    Vercel.com (Free Plan)
        ┌─────────────────────────────────────┐
        │                                     │
        │  HTTPS + HTTP/2 (Auto)             │
        │  CDN Global Distribution           │
        │                                     │
        ├─────────────────────────────────────┤
        │  React 19.2.0 Application          │
        │  ├─ Pages (Dashboard, Risks, etc)  │
        │  ├─ Components (React)             │
        │  ├─ State Management (Zustand)     │
        │  ├─ Routing (React Router)         │
        │  └─ Styling (TailwindCSS)          │
        │                                     │
        ├─────────────────────────────────────┤
        │  API Client Layer                  │
        │  ├─ Axios HTTP client              │
        │  ├─ JWT token management           │
        │  ├─ CORS handling                  │
        │  └─ Error handling                 │
        │                                     │
        ├─────────────────────────────────────┤
        │  Build Process                     │
        │  ├─ Vite build system              │
        │  ├─ TypeScript compilation         │
        │  ├─ Bundle minification            │
        │  └─ Source maps (disabled prod)    │
        │                                     │
        ├─────────────────────────────────────┤
        │  Deployment                        │
        │  ├─ Git push → automatic deploy    │
        │  ├─ Build time: 2-3 minutes        │
        │  ├─ Zero downtime deploys          │
        │  └─ Instant rollback option        │
        │                                     │
        └─────────────────────────────────────┘
               │
               │ HTTPS API Calls
               │ (JSON payloads)
               │
               ▼
```

### 2️⃣ Backend API Layer (Render.com)

```
                 Render.com Web Service (Free Plan)
        ┌──────────────────────────────────────┐
        │                                      │
        │  HTTPS Endpoint                     │
        │  Auto-renewal certificates         │
        │                                      │
        ├──────────────────────────────────────┤
        │  Go 1.26 Application              │
        │  ├─ Fiber v2.52 Web Framework      │
        │  ├─ RESTful API endpoints           │
        │  ├─ Middleware (CORS, Auth, etc)   │
        │  ├─ Business Logic (Services)      │
        │  └─ Data Validation                │
        │                                      │
        ├──────────────────────────────────────┤
        │  Authentication & Security          │
        │  ├─ JWT token validation            │
        │  ├─ CORS middleware                 │
        │  ├─ Rate limiting                   │
        │  ├─ Input validation                │
        │  └─ SQL injection prevention        │
        │                                      │
        ├──────────────────────────────────────┤
        │  Database Layer                     │
        │  ├─ GORM ORM                        │
        │  ├─ Connection pooling              │
        │  ├─ Prepared statements             │
        │  └─ Transaction management          │
        │                                      │
        ├──────────────────────────────────────┤
        │  Docker Container                   │
        │  ├─ Multi-stage build               │
        │  ├─ Alpine Linux (minimal)          │
        │  ├─ Health checks                   │
        │  └─ Graceful shutdown               │
        │                                      │
        ├──────────────────────────────────────┤
        │  Deployment                         │
        │  ├─ Git push → Docker build         │
        │  ├─ Build time: 3-5 minutes         │
        │  ├─ Free tier: 15min sleep timeout │
        │  └─ Auto-restart on crash           │
        │                                      │
        └──────────────────────────────────────┘
               │              │
               │              │
        TCP/Port 5432   TCP/Port 6379
               │              │
               ▼              ▼
```

### 3️⃣ Data Layer

#### PostgreSQL Database (Supabase)

```
        Supabase PostgreSQL (Free Plan)
    ┌───────────────────────────────────┐
    │  Database: openrisk                │
    │  Size: 500 MB available            │
    │  Monthly transfer: 2 GB            │
    │                                    │
    ├────────────────────────────────────┤
    │  Tables:                           │
    │  ├─ users (authentication)         │
    │  ├─ risks (main data)              │
    │  ├─ mitigations (risk actions)     │
    │  ├─ assets (risk assets)           │
    │  ├─ custom_fields (schema extend)  │
    │  ├─ teams (organization)           │
    │  ├─ audit_logs (compliance)        │
    │  └─ ... (other tables)             │
    │                                    │
    ├────────────────────────────────────┤
    │  Features:                         │
    │  ├─ Automatic backups              │
    │  ├─ Point-in-time recovery         │
    │  ├─ MVCC (concurrency)             │
    │  ├─ Full-text search               │
    │  └─ Replication ready              │
    │                                    │
    └────────────────────────────────────┘
```

#### Redis Cache (Redis Cloud)

```
        Redis Cloud (Free Plan)
    ┌──────────────────────────┐
    │  Database: openrisk-cache │
    │  Size: 30 MB available    │
    │  Eviction: LRU            │
    │                           │
    ├──────────────────────────┤
    │  Purpose:                 │
    │  ├─ Session storage       │
    │  ├─ Cache hits            │
    │  ├─ Rate limiting         │
    │  └─ Temporary data        │
    │                           │
    └──────────────────────────┘
```

## Flux de données - Exemple: Login Utilisateur

```
1. USER INTERACTION
   │
   ├─ Enter credentials → Frontend (React)
   │
   └─ Click "Login" button
                │
                ▼
2. FRONTEND PROCESSING
   │
   ├─ Form validation (Zod)
   ├─ Hash password (bcrypt)
   ├─ Create POST request (axios)
   │
   └─ Send HTTPS request
      POST /api/v1/auth/login
         ↓
                │
                ▼
3. VERCEL (GLOBAL CDN)
   │
   ├─ Route request to backend
   │
   └─ Maintain HTTPS connection
                │
                ▼
4. BACKEND PROCESSING (Render)
   │
   ├─ CORS middleware check
   ├─ Rate limit check (Redis)
   ├─ Request validation
   ├─ Extract credentials
   │
   ├─ Database query (PostgreSQL)
   │  SELECT * FROM users WHERE email = ?
   │
   ├─ Verify password (bcrypt)
   ├─ Generate JWT token
   ├─ Cache session (Redis)
   │
   └─ Return JWT token
      HTTPS Response
         ↓
                │
                ▼
5. FRONTEND PROCESSING
   │
   ├─ Parse JWT response
   ├─ Store token (localStorage)
   ├─ Save user info (Zustand state)
   │
   └─ Redirect to dashboard
                │
                ▼
6. DASHBOARD LOAD
   │
   ├─ Send GET /api/v1/risks
      Header: Authorization: Bearer JWT_TOKEN
   │
   ├─ Backend validates token
   ├─ Fetch data (PostgreSQL)
   ├─ Return risks JSON
   │
   └─ Frontend renders dashboard
```

## Infrastructure Stack - Technology Matrix

```
LAYER           TECHNOLOGY          VERSION        STATUS
═════════════════════════════════════════════════════════════════
Frontend        React               19.2.0         ✅ Latest
                Vite                5.1.1          ✅ Latest
                TailwindCSS         3.4.0          ✅ Latest
                TypeScript          5.x            ✅ Latest
                Zustand (state)     5.0.8          ✅ Latest
                Axios (HTTP)        1.13.2         ✅ Latest

Backend         Go                  1.26           ✅ Supported
                Fiber               2.52.10        ✅ Latest
                GORM (ORM)          1.31.1         ✅ Latest
                JWT (auth)          5.3.0          ✅ Latest
                PostgreSQL driver   1.10.9         ✅ Compatible

Database        PostgreSQL          15-alpine      ✅ Cloud managed
                Redis               7-alpine       ✅ Cloud managed

Infrastructure  Docker              Latest         ✅ Containerized
                Render.com          -              ✅ Free hosting
                Vercel              -              ✅ Free hosting
                Supabase            -              ✅ Free DBaaS
                Redis Cloud         -              ✅ Free cache
```

## Limites et Contraintes

```
SERVICE          LIMIT               IMPACT              SOLUTION
════════════════════════════════════════════════════════════════════════
Render.com       15min sleep         API not responsive  uptimerobot.com
                 Free tier           for 30-60 sec       ping service

Vercel           100GB/month         High traffic may    Optimize images
                 bandwidth           exceed limit        Use CDN

Supabase         500 MB storage      Database fills up   Archive old data
                 2GB/month transfer  with time           Delete old risks

Redis Cloud      30 MB cache         Memory overflow     Limit sessions
                 RAM                 if many users       Clear cache

GitHub           Public repo only    Code is public      Accept or use
                 for free auto-deploy                   Enterprise plan
```

## Deployment Pipeline - CI/CD

```
Developer writes code
      ↓
    git push
      ↓
GitHub receives push
      ├─ Trigger Render webhook
      │  ├─ Pull latest code
      │  ├─ Build Docker image (3-5 min)
      │  ├─ Run tests
      │  ├─ Deploy new container
      │  └─ Health check
      │
      └─ Trigger Vercel webhook
         ├─ Pull latest code
         ├─ Install dependencies
         ├─ Build frontend (2-3 min)
         ├─ Run tests
         ├─ Deploy to CDN
         └─ Invalidate cache
              ↓
         ✅ Both services live
```

## Monitoring Points

```
COMPONENT           CHECK POINT         FREQUENCY       ACTION
════════════════════════════════════════════════════════════════════════
Render Backend      /api/health         Every 14min      Keep awake
Vercel Frontend     Load time           24 hours         Performance
Supabase DB         Storage usage       Daily            Archive data
Redis Cache         Memory usage        Daily            Clear cache
Error logs          Backend logs        Real-time        Alert on error
Performance         Response time       Hourly           Optimize
```

## High Availability Considerations

Current architecture:
- ✅ Frontend: Global CDN (99.99% uptime)
- ✅ Backend: Single region (99.9% uptime)
- ✅ Database: Single region (99.9% uptime)

For production upgrade:
- Add backup backend on different region
- Enable Supabase replication
- Implement Redis clustering
- Add load balancing

## Security Architecture

```
                    HTTPS/TLS
                 ┌───────────┐
                 │ Encryption│
                 └─────┬─────┘
                       │
        ┌──────────────┼──────────────┐
        │              │              │
        ▼              ▼              ▼
    JWT Auth     CORS Check    Rate Limiting
        │              │              │
        └──────────────┼──────────────┘
                       │
                  Input Valid.
                  SQL Injection
                  Prevention
                       │
                  ✅ Safe DB Query
```

---

## Résumé

✅ **Frontend**: Vercel (Global CDN, Auto-deploy, Free HTTPS)
✅ **Backend**: Render.com (Docker, Auto-deploy, Free HTTPS)
✅ **Database**: Supabase (PostgreSQL, 500MB, Managed)
✅ **Cache**: Redis Cloud (30MB, Managed)
✅ **CI/CD**: GitHub (Auto-deploy on push)

**Total Cost**: $0.00/month 💰
**Availability**: 99.9% uptime
**Scalability**: Ready to scale when needed
