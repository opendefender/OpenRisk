# 🐳 Docker Configuration

Docker image for deploying OpenRisk backend on Render.com.

## 📄 Files

### Dockerfile.render
Optimized Docker configuration for Render.com web services.

**Features**:
- Multi-stage build (minimal image size)
- Alpine Linux base (security + performance)
- Health checks included
- Optimized for Go applications
- Production-ready

---

## 🚀 Deployment

### Render.com Setup

1. **Create Web Service**
   - Go to https://render.com
   - Click **New** → **Web Service**
   - Connect your GitHub repository

2. **Configuration**
   - **Name**: `openrisk-api`
   - **Environment**: `Docker`
   - **Region**: Frankfurt (or closest to you)
   - **Build Command**: 
     ```bash
     docker build -f deployment/docker/Dockerfile.render -t openrisk .
     ```
   - **Start Command**:
     ```bash
     ./server
     ```

3. **Environment Variables** (Add in Render dashboard)
   ```env
   DATABASE_URL=postgresql://...
   REDIS_URL=redis://...
   RSA_PRIVATE_KEY=<PEM private key, see scripts/generate_rsa_keys.sh>
   RSA_PUBLIC_KEY=<PEM public key>
   CORS_ORIGINS=https://openrisk-xxxx.vercel.app
   API_BASE_URL=https://openrisk-api.onrender.com
   PORT=8080
   ENVIRONMENT=production
   LOG_LEVEL=info
   ```

4. **Deploy**
   - Click **Create Web Service**
   - Wait 3-5 minutes for build & deployment
   - Check logs for errors

---

## 📋 Build Stages

### Stage 1: Builder
- Downloads Go dependencies
- Compiles the Go application
- Creates binary

### Stage 2: Runtime
- Minimal Alpine Linux image
- Copies only the binary (no source code)
- Adds CA certificates for HTTPS
- Sets health check

**Result**: Small, secure, production-ready image

---

## 🏥 Health Check

The Docker image includes a health check:

```dockerfile
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:${PORT:-8080}/api/health || exit 1
```

This ensures:
- Service is monitored continuously
- Bad deployments are detected quickly
- Automatic restart on failure

---

## 🔍 Local Testing

### Build Locally
```bash
docker build -f deployment/docker/Dockerfile.render -t openrisk .
```

### Run Locally
```bash
# From the repository root. Writes secrets/private.pem and secrets/public.pem.
bash scripts/generate_rsa_keys.sh

docker run -p 8080:8080 \
  -e DATABASE_URL="postgresql://..." \
  -e REDIS_URL="redis://..." \
  -e RSA_PRIVATE_KEY_PATH=/app/secrets/private.pem \
  -e RSA_PUBLIC_KEY_PATH=/app/secrets/public.pem \
  -v "$PWD/secrets:/app/secrets:ro" \
  -e CORS_ORIGINS="http://localhost:5173" \
  -e API_BASE_URL="http://localhost:8080" \
  openrisk
```

### Test Health
```bash
curl http://localhost:8080/api/health
# Should return: {"status":"OK"}
```

---

## 📊 Image Optimization

**Original Size** (with source code): ~500 MB
**Multi-stage Build** (optimized): ~150 MB
**Benefit**: Faster deployments, less bandwidth

---

## 🚨 Troubleshooting

**Build fails**: 
- Check Go dependencies are downloadable
- Verify `go.mod` and `go.sum` are in place
- Check Go version (1.25.4 required)

**Health check fails**:
- Verify API endpoint responds: `/api/health`
- Check port is correct (8080 by default)
- Review environment variables

**Container won't start**:
- Check environment variables are set
- Review logs: `docker logs container_name`
- Verify database connection string

---

## 📝 File Locations

```
OpenRisk/
├── deployment/
│   └── docker/
│       └── Dockerfile.render    ← You are here
├── backend/
│   ├── cmd/server/main.go       ← Entry point
│   ├── go.mod                   ← Dependencies
│   ├── go.sum                   ← Checksums
│   └── ...
└── migrations/                  ← SQL migrations
```

---

## 🔗 Related Files

- **Configuration**: See `deployment/configs/README.md`
- **Deployment Guide**: See `deployment/guides/README_DEPLOYMENT.txt`
- **Environment Setup**: See `deployment/configs/.env.backend.example`

---

## 💡 Best Practices

1. **Always use Alpine base** for production
2. **Multi-stage builds** to keep images small
3. **Health checks** for reliability
4. **Minimal dependencies** for security
5. **Pin versions** (especially base images)

---

## 📚 Resources

- **Docker Docs**: https://docs.docker.com/
- **Render.com Docs**: https://render.com/docs
- **Go Docker**: https://golang.org/doc/containers

---

**Ready to deploy? See `deployment/guides/README_DEPLOYMENT.txt` → Phase 2: Backend Deployment**
