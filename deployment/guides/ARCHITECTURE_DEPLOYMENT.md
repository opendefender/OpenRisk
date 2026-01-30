 🏗 Architecture de d�ploiement OpenRisk

 Diagramme global


                        🌍 INTERNET 🌍
                        
    User Browser          Mobile App         API Clients
           │                  │                    │
           └──────────────────┼────────────────────┘
                              │
                        HTTPS (TLS/SSL)
                              │
      ╔═════════════════════════════════════════╗
      ║          � VERCEL CDN GLOBAL           ║
      ║   https://openrisk-xxxx.vercel.app     ║
      ║                                         ║
      ║  Frontend (React + Vite + TailwindCSS) ║
      ║   Auto-deploy from GitHub            ║
      ║   Global CDN                         ║
      ║   GB/mois bandwidth               ║
      ║   HTTPS automatic                    ║
      ╚═════════════════════════════════════════╝
                              │
                              │ HTTPS API Calls
                              │ (JSON REST)
                              ▼
      ╔═════════════════════════════════════════╗
      ║      � RENDER.COM - BACKEND           ║
      ║  https://openrisk-api.onrender.com     ║
      ║                                         ║
      ║  Go .. + Fiber API Server          ║
      ║   Docker container                   ║
      ║   Auto-deploy from GitHub            ║
      ║   Free tier with min sleep         ║
      ║   HTTPS automatic                    ║
      ╚═════════════════════════════════════════╝
                              │
                ┌─────────────┼─────────────┐
                │             │             │
           TCP/IP         TCP/IP         TCP/IP
                │             │             │
                ▼             ▼             ▼
    ╔═══════════════════╗ ╔═════════════╗ ╔══════════════╗
    ║   🟪 SUPABASE     ║ ║ � REDIS    ║ ║  LOGS      ║
    ║                 ║ ║ CLOUD       ║ ║              ║
    ║  PostgreSQL DB  ║ ║             ║ ║ Server Logs  ║
    ║   MB Storage ║ ║  MB Cache ║ ║ Request Logs ║
    ║  GB trans/mo   ║ ║ Sessions    ║ ║              ║
    ║                 ║ ║ Caching     ║ ║ Render/Vercel║
    ╚═══════════════════╝ ╚═════════════╝ ╚══════════════╝


 Architecture d�taill�e par composant

 ⃣ Frontend Layer (Vercel)


                    Vercel.com (Free Plan)
        ┌─────────────────────────────────────┐
        │                                     │
        │  HTTPS + HTTP/ (Auto)             │
        │  CDN Global Distribution           │
        │                                     │
        ├─────────────────────────────────────┤
        │  React .. Application          │
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
        │  ├─ Build time: - minutes        │
        │  ├─ Zero downtime deploys          │
        │  └─ Instant rollback option        │
        │                                     │
        └─────────────────────────────────────┘
               │
               │ HTTPS API Calls
               │ (JSON payloads)
               │
               ▼


 ⃣ Backend API Layer (Render.com)


                 Render.com Web Service (Free Plan)
        ┌──────────────────────────────────────┐
        │                                      │
        │  HTTPS Endpoint                     │
        │  Auto-renewal certificates         │
        │                                      │
        ├──────────────────────────────────────┤
        │  Go .. Application              │
        │  ├─ Fiber v. Web Framework      │
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
        │  ├─ Build time: - minutes         │
        │  ├─ Free tier: min sleep timeout │
        │  └─ Auto-restart on crash           │
        │                                      │
        └──────────────────────────────────────┘
               │              │
               │              │
        TCP/Port    TCP/Port 
               │              │
               ▼              ▼


 ⃣ Data Layer

 PostgreSQL Database (Supabase)


        Supabase PostgreSQL (Free Plan)
    ┌───────────────────────────────────┐
    │  Database: openrisk                │
    │  Size:  MB available            │
    │  Monthly transfer:  GB            │
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


 Redis Cache (Redis Cloud)


        Redis Cloud (Free Plan)
    ┌──────────────────────────┐
    │  Database: openrisk-cache │
    │  Size:  MB available    │
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


 Flux de donn�es - Exemple: Login Utilisateur


. USER INTERACTION
   │
   ├─ Enter credentials → Frontend (React)
   │
   └─ Click "Login" button
                │
                ▼
. FRONTEND PROCESSING
   │
   ├─ Form validation (Zod)
   ├─ Hash password (bcrypt)
   ├─ Create POST request (axios)
   │
   └─ Send HTTPS request
      POST /api/v/auth/login
         ↓
                │
                ▼
. VERCEL (GLOBAL CDN)
   │
   ├─ Route request to backend
   │
   └─ Maintain HTTPS connection
                │
                ▼
. BACKEND PROCESSING (Render)
   │
   ├─ CORS middleware check
   ├─ Rate limit check (Redis)
   ├─ Request validation
   ├─ Extract credentials
   │
   ├─ Database query (PostgreSQL)
   │  SELECT  FROM users WHERE email = ?
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
. FRONTEND PROCESSING
   │
   ├─ Parse JWT response
   ├─ Store token (localStorage)
   ├─ Save user info (Zustand state)
   │
   └─ Redirect to dashboard
                │
                ▼
. DASHBOARD LOAD
   │
   ├─ Send GET /api/v/risks
      Header: Authorization: Bearer JWT_TOKEN
   │
   ├─ Backend validates token
   ├─ Fetch data (PostgreSQL)
   ├─ Return risks JSON
   │
   └─ Frontend renders dashboard


 Infrastructure Stack - Technology Matrix


LAYER           TECHNOLOGY          VERSION        STATUS
═════════════════════════════════════════════════════════════════
Frontend        React               ..          Latest
                Vite                ..           Latest
                TailwindCSS         ..           Latest
                TypeScript          .x             Latest
                Zustand (state)     ..           Latest
                Axios (HTTP)        ..          Latest

Backend         Go                  ..          Latest
                Fiber               ..         Latest
                GORM (ORM)          ..          Latest
                JWT (auth)          ..           Latest
                PostgreSQL driver   ..          Compatible

Database        PostgreSQL          -alpine       Cloud managed
                Redis               -alpine        Cloud managed

Infrastructure  Docker              Latest          Containerized
                Render.com          -               Free hosting
                Vercel              -               Free hosting
                Supabase            -               Free DBaaS
                Redis Cloud         -               Free cache


 Limites et Contraintes


SERVICE          LIMIT               IMPACT              SOLUTION
════════════════════════════════════════════════════════════════════════
Render.com       min sleep         API not responsive  uptimerobot.com
                 Free tier           for - sec       ping service

Vercel           GB/month         High traffic may    Optimize images
                 bandwidth           exceed limit        Use CDN

Supabase          MB storage      Database fills up   Archive old data
                 GB/month transfer  with time           Delete old risks

Redis Cloud       MB cache         Memory overflow     Limit sessions
                 RAM                 if many users       Clear cache

GitHub           Public repo only    Code is public      Accept or use
                 for free auto-deploy                   Enterprise plan


 Deployment Pipeline - CI/CD


Developer writes code
      ↓
    git push
      ↓
GitHub receives push
      ├─ Trigger Render webhook
      │  ├─ Pull latest code
      │  ├─ Build Docker image (- min)
      │  ├─ Run tests
      │  ├─ Deploy new container
      │  └─ Health check
      │
      └─ Trigger Vercel webhook
         ├─ Pull latest code
         ├─ Install dependencies
         ├─ Build frontend (- min)
         ├─ Run tests
         ├─ Deploy to CDN
         └─ Invalidate cache
              ↓
          Both services live


 Monitoring Points


COMPONENT           CHECK POINT         FREQUENCY       ACTION
════════════════════════════════════════════════════════════════════════
Render Backend      /api/health         Every min      Keep awake
Vercel Frontend     Load time            hours         Performance
Supabase DB         Storage usage       Daily            Archive data
Redis Cache         Memory usage        Daily            Clear cache
Error logs          Backend logs        Real-time        Alert on error
Performance         Response time       Hourly           Optimize


 High Availability Considerations

Current architecture:
-  Frontend: Global CDN (.% uptime)
-  Backend: Single region (.% uptime)
-  Database: Single region (.% uptime)

For production upgrade:
- Add backup backend on different region
- Enable Supabase replication
- Implement Redis clustering
- Add load balancing

 Security Architecture


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
                   Safe DB Query


---

 R�sum�

 Frontend: Vercel (Global CDN, Auto-deploy, Free HTTPS)
 Backend: Render.com (Docker, Auto-deploy, Free HTTPS)
 Database: Supabase (PostgreSQL, MB, Managed)
 Cache: Redis Cloud (MB, Managed)
 CI/CD: GitHub (Auto-deploy on push)

Total Cost: $./month �
Availability: .% uptime
Scalability: Ready to scale when needed
