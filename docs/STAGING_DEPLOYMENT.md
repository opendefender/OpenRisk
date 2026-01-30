 Staging Environment Deployment Guide

 Overview

This guide covers deploying OpenRisk to a staging environment for pre-production testing and validation.

 Architecture

 Staging Environment Components



                  Staging Environment                    

                                                          
     
           Docker Compose (Primary)                   
     
    - PostgreSQL  (staging_db)                      
    - Redis  (staging_redis)                         
    - Backend API (Go Fiber)                          
    - Frontend (React + Vite)                         
    - Nginx (Reverse Proxy)                           
     
                                                          
     
         External Services (Optional)                 
     
    - TheHive (for incident sync testing)             
    - OpenCTI (for threat intelligence)               
     
                                                          



 Prerequisites

 Server Requirements

- OS: Linux (Ubuntu .+, CentOS +, or equivalent)
- CPU: + cores (+ recommended)
- RAM: + GB (+ recommended)
- Storage: + GB SSD
- Network: Public IP, open ports / for HTTPS

 Software Requirements

- Docker .+
- Docker Compose .+
- Git
- Nginx (if using reverse proxy)
- SSL certificate (Let's Encrypt recommended)

 DNS Configuration


staging.openrisk.yourdomain.com  → <server-ip>
api.staging.openrisk.yourdomain.com  → <server-ip>


 Deployment Steps

 . Server Preparation

bash
 Update system packages
sudo apt-get update && sudo apt-get upgrade -y

 Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

 Install Docker Compose
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose

 Add user to docker group (optional, for non-root execution)
sudo usermod -aG docker $USER

 Verify installation
docker --version
docker-compose --version


 . Clone & Setup Repository

bash
 Clone the repository
git clone https://github.com/opendefender/openrisk.git
cd openrisk

 Checkout staging branch (or main)
git checkout stag

 Create deployment directory
sudo mkdir -p /opt/openrisk-staging
sudo chown $USER:$USER /opt/openrisk-staging

 Copy files to deployment directory
cp -r . /opt/openrisk-staging
cd /opt/openrisk-staging


 . Configure Environment

bash
 Copy environment template
cp .env.example .env.staging

 Edit configuration for staging
nano .env.staging


Key configuration values:

env
 Server
PORT=
APP_ENV=staging
JWT_SECRET=<generate-with-openssl-rand-base->

 Database (use strong password!)
DB_USER=openrisk_stag
DB_PASSWORD=<strong-random-password>
DB_NAME=openrisk_staging

 CORS (for staging domain)
CORS_ORIGINS=https://staging.openrisk.yourdomain.com,https://api.staging.openrisk.yourdomain.com

 Redis
REDIS_URL=redis://redis:/

 Frontend
VITE_API_URL=https://api.staging.openrisk.yourdomain.com/api

 Logging
LOG_LEVEL=info
LOG_FORMAT=json

 Optional: External integrations
THEHIVE_ENABLED=true
THEHIVE_URL=https://thehive.yourdomain.com
THEHIVE_API_KEY=<your-api-key>


 . Prepare Docker Compose for Staging

Create docker-compose.staging.yml:

yaml
version: '.'

services:
  db:
    image: postgres:-alpine
    container_name: openrisk_staging_db
    environment:
      POSTGRES_USER: ${DB_USER}
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_DB: ${DB_NAME}
    volumes:
      - openrisk_staging_data:/var/lib/postgresql/data
      - ./backup:/backup
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${DB_USER}"]
      interval: s
      timeout: s
      retries: 
    networks:
      - openrisk_staging_network
    restart: always

  redis:
    image: redis:-alpine
    container_name: openrisk_staging_redis
    command: redis-server --appendonly yes --requirepass ${REDIS_PASSWORD:-}
    volumes:
      - openrisk_staging_redis:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: s
      timeout: s
      retries: 
    networks:
      - openrisk_staging_network
    restart: always

  backend:
    build:
      context: ./backend
      dockerfile: Dockerfile
    container_name: openrisk_staging_backend
    depends_on:
      db:
        condition: service_healthy
      redis:
        condition: service_healthy
    environment:
      DB_HOST: db
      DB_PORT: 
      DB_USER: ${DB_USER}
      DB_PASSWORD: ${DB_PASSWORD}
      DB_NAME: ${DB_NAME}
      PORT: 
      JWT_SECRET: ${JWT_SECRET}
      CORS_ORIGINS: ${CORS_ORIGINS}
      REDIS_URL: redis://redis:/
      APP_ENV: staging
      LOG_LEVEL: info
    expose:
      - 
    networks:
      - openrisk_staging_network
    restart: always
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:/api/v/health"]
      interval: s
      timeout: s
      retries: 

  frontend:
    build:
      context: ./frontend
      dockerfile: Dockerfile
    container_name: openrisk_staging_frontend
    depends_on:
      - backend
    environment:
      VITE_API_URL: ${VITE_API_URL}
      NODE_ENV: production
    expose:
      - 
    networks:
      - openrisk_staging_network
    restart: always
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:"]
      interval: s
      timeout: s
      retries: 

  nginx:
    image: nginx:alpine
    container_name: openrisk_staging_nginx
    depends_on:
      - backend
      - frontend
    ports:
      - ":"
      - ":"
    volumes:
      - ./nginx/staging.conf:/etc/nginx/nginx.conf:ro
      - ./certs:/etc/nginx/certs:ro
      - openrisk_staging_logs:/var/log/nginx
    networks:
      - openrisk_staging_network
    restart: always

volumes:
  openrisk_staging_data:
  openrisk_staging_redis:
  openrisk_staging_logs:

networks:
  openrisk_staging_network:
    driver: bridge


 . Configure Nginx Reverse Proxy

Create nginx/staging.conf:

nginx
events {
    worker_connections ;
}

http {
    upstream backend {
        server backend:;
    }

    upstream frontend {
        server frontend:;
    }

    server {
        listen ;
        server_name staging.openrisk.yourdomain.com;
        
         Redirect HTTP to HTTPS
        return  https://$server_name$request_uri;
    }

    server {
        listen  ssl http;
        server_name staging.openrisk.yourdomain.com;

         SSL Certificate (Let's Encrypt)
        ssl_certificate /etc/nginx/certs/fullchain.pem;
        ssl_certificate_key /etc/nginx/certs/privkey.pem;

         SSL Configuration
        ssl_protocols TLSv. TLSv.;
        ssl_ciphers HIGH:!aNULL:!MD;
        ssl_prefer_server_ciphers on;

         Security headers
        add_header Strict-Transport-Security "max-age=; includeSubDomains" always;
        add_header X-Content-Type-Options "nosniff" always;
        add_header X-Frame-Options "DENY" always;
        add_header X-XSS-Protection "; mode=block" always;

         Frontend
        location / {
            proxy_pass http://frontend;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
        }

         Backend API
        location /api/ {
            proxy_pass http://backend/api/;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            
             WebSocket support
            proxy_http_version .;
            proxy_set_header Upgrade $http_upgrade;
            proxy_set_header Connection "upgrade";
        }

         Health check
        location /health {
            access_log off;
            proxy_pass http://backend/api/v/health;
        }
    }
}


 . SSL Certificate Setup (Let's Encrypt)

bash
 Install Certbot
sudo apt-get install certbot python-certbot-nginx -y

 Create certificate directory
sudo mkdir -p /opt/openrisk-staging/certs

 Generate certificate
sudo certbot certonly --standalone \
  -d staging.openrisk.yourdomain.com \
  -d api.staging.openrisk.yourdomain.com \
  --email admin@yourdomain.com

 Copy certificates
sudo cp /etc/letsencrypt/live/staging.openrisk.yourdomain.com/fullchain.pem /opt/openrisk-staging/certs/
sudo cp /etc/letsencrypt/live/staging.openrisk.yourdomain.com/privkey.pem /opt/openrisk-staging/certs/
sudo chown $USER:$USER /opt/openrisk-staging/certs/

 Auto-renewal
sudo systemctl enable certbot.timer
sudo systemctl start certbot.timer


 . Start Services

bash
 Navigate to deployment directory
cd /opt/openrisk-staging

 Build and start containers
docker-compose -f docker-compose.staging.yml up -d

 Verify services are running
docker-compose -f docker-compose.staging.yml ps

 Expected output:
 NAME                          STATUS
 openrisk_staging_db           Up (healthy)
 openrisk_staging_redis        Up (healthy)
 openrisk_staging_backend      Up (healthy)
 openrisk_staging_frontend     Up (healthy)
 openrisk_staging_nginx        Up


 . Initialize Database

bash
 Connect to database container
docker-compose -f docker-compose.staging.yml exec db psql -U openrisk_stag -d openrisk_staging

 Run migrations manually if needed
docker-compose -f docker-compose.staging.yml exec backend openrisk migrate up

 Seed with test data (optional)
docker-compose -f docker-compose.staging.yml exec backend openrisk seed


 . Verify Deployment

bash
 Check service health
curl https://staging.openrisk.yourdomain.com/health

 Check API
curl https://api.staging.openrisk.yourdomain.com/api/v/health

 Check logs
docker-compose -f docker-compose.staging.yml logs -f

 Test frontend
 Open browser: https://staging.openrisk.yourdomain.com


 Post-Deployment Tasks

 . Monitoring Setup

bash
 View container logs
docker-compose -f docker-compose.staging.yml logs backend
docker-compose -f docker-compose.staging.yml logs frontend

 Monitor resource usage
docker stats

 Set up log aggregation (optional)
 Configure ELK stack or Grafana for monitoring


 . Backup Configuration

bash
 Create backup script
cat > /opt/openrisk-staging/backup.sh << 'EOF'
!/bin/bash
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR="/opt/openrisk-staging/backup"

 Create backup directory
mkdir -p $BACKUP_DIR

 Backup database
docker-compose -f docker-compose.staging.yml exec db pg_dump \
  -U openrisk_stag openrisk_staging > "$BACKUP_DIR/db_${DATE}.sql"

 Backup files
tar -czf "$BACKUP_DIR/files_${DATE}.tar.gz" \
  --exclude=node_modules \
  --exclude=dist \
  --exclude=vendor \
  /opt/openrisk-staging

echo " Backup completed: $BACKUP_DIR"
EOF

chmod +x /opt/openrisk-staging/backup.sh

 Schedule daily backups
(crontab -l >/dev/null; echo "     /opt/openrisk-staging/backup.sh") | crontab -


 . Security Hardening

bash
 Set file permissions
chmod  /opt/openrisk-staging/.env.staging
chmod  /opt/openrisk-staging/docker-compose.staging.yml

 Enable firewall
sudo ufw enable
sudo ufw allow /tcp
sudo ufw allow /tcp
sudo ufw allow /tcp

 Disable unused ports
sudo ufw default deny incoming
sudo ufw default allow outgoing


 . Performance Tuning

bash
 PostgreSQL configuration
docker-compose -f docker-compose.staging.yml exec db psql -U openrisk_stag -d openrisk_staging << 'EOF'
ALTER SYSTEM SET shared_buffers = 'MB';
ALTER SYSTEM SET effective_cache_size = 'GB';
ALTER SYSTEM SET maintenance_work_mem = 'MB';
ALTER SYSTEM SET work_mem = 'MB';
SELECT pg_reload_conf();
EOF

 Redis configuration
 Edit redis config for memory limits and persistence


 Maintenance & Operations

 Daily Checks

bash
 Verify all services are healthy
docker-compose -f docker-compose.staging.yml ps

 Check disk usage
df -h /opt/openrisk-staging

 Check logs for errors
docker-compose -f docker-compose.staging.yml logs --since h | grep ERROR


 Updating to New Versions

bash
 Pull latest code
cd /opt/openrisk-staging
git fetch origin
git checkout stag
git pull origin stag

 Rebuild containers
docker-compose -f docker-compose.staging.yml build

 Restart services
docker-compose -f docker-compose.staging.yml down
docker-compose -f docker-compose.staging.yml up -d

 Verify deployment
curl https://staging.openrisk.yourdomain.com/health


 Database Maintenance

bash
 Backup before maintenance
/opt/openrisk-staging/backup.sh

 Vacuum database (optimize)
docker-compose -f docker-compose.staging.yml exec db \
  psql -U openrisk_stag -d openrisk_staging -c "VACUUM ANALYZE;"

 Check database size
docker-compose -f docker-compose.staging.yml exec db \
  psql -U openrisk_stag -d openrisk_staging -c "SELECT pg_size_pretty(pg_database_size('openrisk_staging'));"


 Troubleshooting

 Service won't start

bash
 Check logs
docker-compose -f docker-compose.staging.yml logs <service-name>

 Verify ports are available
lsof -i :
lsof -i :
lsof -i :

 Restart service
docker-compose -f docker-compose.staging.yml restart <service-name>


 Database connection issues

bash
 Test connectivity
docker-compose -f docker-compose.staging.yml exec backend \
  pg_isready -h db -p 

 Check credentials
docker-compose -f docker-compose.staging.yml exec db psql -U openrisk_stag -c "\conninfo"


 SSL certificate issues

bash
 Verify certificate
openssl x -in /opt/openrisk-staging/certs/fullchain.pem -text -noout

 Renew certificate
sudo certbot renew --force-renewal

 Verify Nginx config
docker-compose -f docker-compose.staging.yml exec nginx nginx -t


 Performance Baseline

| Metric | Target | Current |
|--------|--------|---------|
| Response Time | <ms | TBD |
| Throughput | > req/s | TBD |
| Error Rate | <.% | TBD |
| CPU Usage | <% | TBD |
| Memory Usage | <% | TBD |

 Production Readiness Checklist

- [ ] All health checks passing
- [ ] Database backups configured
- [ ] SSL certificate valid
- [ ] Firewall configured
- [ ] Log aggregation setup
- [ ] Monitoring alerts configured
- [ ] Load testing completed
- [ ] Security audit passed
- [ ] Documentation updated
- [ ] Team trained on operations

 Next Steps

. Load Testing: Run performance tests against staging
. Security Audit: Conduct penetration testing
. User Acceptance Testing: Have stakeholders validate features
. Production Preparation: Document runbooks and incident procedures
. Go-Live Planning: Schedule production deployment

---

Questions? See docs/ for additional guides or create a GitHub issue.
