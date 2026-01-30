  Docker Configuration

Docker image for deploying OpenRisk backend on Render.com.

  Files

 Dockerfile.render
Optimized Docker configuration for Render.com web services.

Features:
- Multi-stage build (minimal image size)
- Alpine Linux base (security + performance)
- Health checks included
- Optimized for Go applications
- Production-ready

---

  Deployment

 Render.com Setup

. Create Web Service
   - Go to https://render.com
   - Click New → Web Service
   - Connect your GitHub repository

. Configuration
   - Name: openrisk-api
   - Environment: Docker
   - Region: Frankfurt (or closest to you)
   - Build Command: 
     bash
     docker build -f deployment/docker/Dockerfile.render -t openrisk .
     
   - Start Command:
     bash
     ./server
     

. Environment Variables (Add in Render dashboard)
   env
   DATABASE_URL=postgresql://...
   REDIS_URL=redis://...
   JWT_SECRET=your--char-secret
   CORS_ORIGINS=https://openrisk-xxxx.vercel.app
   API_BASE_URL=https://openrisk-api.onrender.com
   PORT=
   ENVIRONMENT=production
   LOG_LEVEL=info
   

. Deploy
   - Click Create Web Service
   - Wait - minutes for build & deployment
   - Check logs for errors

---

  Build Stages

 Stage : Builder
- Downloads Go dependencies
- Compiles the Go application
- Creates binary

 Stage : Runtime
- Minimal Alpine Linux image
- Copies only the binary (no source code)
- Adds CA certificates for HTTPS
- Sets health check

Result: Small, secure, production-ready image

---

  Health Check

The Docker image includes a health check:

dockerfile
HEALTHCHECK --interval=s --timeout=s --start-period=s --retries= \
  CMD curl -f http://localhost:${PORT:-}/api/health || exit 


This ensures:
- Service is monitored continuously
- Bad deployments are detected quickly
- Automatic restart on failure

---

  Local Testing

 Build Locally
bash
docker build -f deployment/docker/Dockerfile.render -t openrisk .


 Run Locally
bash
docker run -p : \
  -e DATABASE_URL="postgresql://..." \
  -e REDIS_URL="redis://..." \
  -e JWT_SECRET="your-secret" \
  -e CORS_ORIGINS="http://localhost:" \
  -e API_BASE_URL="http://localhost:" \
  openrisk


 Test Health
bash
curl http://localhost:/api/health
 Should return: {"status":"OK"}


---

  Image Optimization

Original Size (with source code): ~ MB
Multi-stage Build (optimized): ~ MB
Benefit: Faster deployments, less bandwidth

---

  Troubleshooting

Build fails: 
- Check Go dependencies are downloadable
- Verify go.mod and go.sum are in place
- Check Go version (.. required)

Health check fails:
- Verify API endpoint responds: /api/health
- Check port is correct ( by default)
- Review environment variables

Container won't start:
- Check environment variables are set
- Review logs: docker logs container_name
- Verify database connection string

---

  File Locations


OpenRisk/
 deployment/
    docker/
        Dockerfile.render    ← You are here
 backend/
    cmd/server/main.go       ← Entry point
    go.mod                   ← Dependencies
    go.sum                   ← Checksums
    ...
 migrations/                  ← SQL migrations


---

  Related Files

- Configuration: See deployment/configs/README.md
- Deployment Guide: See deployment/guides/README_DEPLOYMENT.txt
- Environment Setup: See deployment/configs/.env.backend.example

---

  Best Practices

. Always use Alpine base for production
. Multi-stage builds to keep images small
. Health checks for reliability
. Minimal dependencies for security
. Pin versions (especially base images)

---

  Resources

- Docker Docs: https://docs.docker.com/
- Render.com Docs: https://render.com/docs
- Go Docker: https://golang.org/doc/containers

---

Ready to deploy? See deployment/guides/README_DEPLOYMENT.txt → Phase : Backend Deployment
