# 🏪 Hardware Store API
- A production-ready, multi-store hardware e-commerce API built with Go, featuring multi-payment support (M-Pesa, Airtel Money, Card), WhatsApp notifications, and geolocation-based store finder.

## ✨ Features
- Multi-Store Architecture - Support for multiple store locations with inventory management

- Payment Processing - M-Pesa, Airtel Money, and Card payments (Pesapal)

- Real-time Notifications - Email (SMTP) and WhatsApp (Green API) support

- Geolocation - Store finder with distance calculations (Haversine)

- Secure - JWT authentication with refresh tokens, AES-256-GCM encryption

- File Storage - Cloudinary integration for images

- Proof of Delivery - POD with photo uploads

- Multi-Currency - Support for different currencies per store

- Caching - Redis for performance optimization

- Containerized - Docker and Docker Compose for easy deployment

## 🚀 Quick Start
### Prerequisites
- Docker (version 20.10+)

- Docker Compose (version 2.0+)

- Git

- Make (optional, for convenience)

## 1. Clone the Repository
bash
git clone https://github.com/Bessmack/hardware-store-api.git
cd hardware-store-api
## 2. Environment Configuration
- Copy the example environment file and configure it:

bash
- cp .env.example .env
- Edit the .env file with your credentials:

bash
nano .env  # or vim, or use your preferred editor
### Required configurations:

- JWT_SECRET - Generate with: openssl rand -base64 32

- ENCRYPTION_KEY - Generate with: openssl rand -hex 32

- DATABASE_URL - Update if using non-default ports

- Payment provider credentials (M-Pesa, Airtel, Pesapal)

## 3. Run with Docker Compose
bash
-#- Build and start all services
-- docker-compose up -d

-#- Watch logs to confirm everything is running
-- docker-compose logs -f api

#### You should see:
##### "INF migrations completed successfully"
##### "INF server listening port=8080"
## 4. Verify Installation
bash
-#- Health check
- curl http://localhost:8080/health

-#- Test API endpoints
- curl http://localhost:8080/api/v1/products

-#- Or use the provided test script
- ./test_endpoints.sh
-- Your API is now running at http://localhost:8080! 🎉

### 📦 Docker Setup Details
#### Services
- The application consists of three services defined in docker-compose.yml:

Service -- Container -- Name -- Port (Host) -- Port (Container) --	Purpose
API	hardware-store-api	8080	8080	Main application server
PostgreSQL	hardware-store-db	5432	5432	Primary database
Redis	hardware-store-cache	6379	6379	Caching and session storage
## Port Conflicts
- If you have existing services running on ports 5432 or 6379, you can change the host ports in docker-compose.yml:

yaml
services:
  postgres:
    ports:
      - "5433:5432"  # Host:Container

  redis:
    ports:
      - "6380:6379"  # Host:Container
- Then update your .env:

env
DATABASE_URL=postgres://postgres:postgres@postgres:5433/hardware_store?sslmode=disable
REDIS_URL=redis://redis:6380
🛠️ Common Docker Commands
Start Services
bash
-#- Start in detached mode (background)
docker-compose up -d

#### Start with logs visible
- docker-compose up
- Stop Services
bash
#### Stop without removing containers
- docker-compose stop

#### Stop and remove containers (preserves data)
- docker-compose down

#### Stop and remove everything (including volumes - WARNING: deletes data!)
docker-compose down -v
View Logs
bash
#### All services
docker-compose logs

#### Specific service (api, postgres, redis)
docker-compose logs -f api

#### Tail last 100 lines
docker-compose logs --tail 100 -f api
Container Management
bash
#### List running containers
docker-compose ps

#### Access container shell
docker-compose exec api sh

#### Run a command in a container
docker-compose exec api ./main migrate

#### Restart a service
docker-compose restart api

#### Rebuild after code changes
docker-compose up -d --build
Database Management
bash
#### Access PostgreSQL CLI
docker-compose exec postgres psql -U postgres -d hardware_store

#### Backup database
docker-compose exec postgres pg_dump -U postgres hardware_store > backup.sql

#### Restore database
cat backup.sql | docker-compose exec -T postgres psql -U postgres -d hardware_store

#### Run migrations manually
docker-compose exec api ./main migrate
Redis Management
bash
#### Access Redis CLI
docker-compose exec redis redis-cli

#### Monitor Redis activity
docker-compose exec redis redis-cli monitor

## 5) Start With Makefile commands:

### 🚀 Quick Usage Examples

#### Start everything
make up

#### Check if everything is working
make health

#### Check migration status
make migrate-status

#### View API logs
make logs api

#### Test the API endpoints
make api-products
make api-stores
make api-categories

#### Stop everything
make down

### Helpful API testing commands:

#### Check API health (now uses /api/v1/health)
make health

#### Get API status with pretty JSON
make api-status

#### Fetch products
make api-products

#### Fetch stores
make api-stores

#### Fetch categories
make api-categories




## Flush cache
docker-compose exec redis redis-cli flushall
🔧 Environment Variables
Required Configuration
Variable	Description	Example
DATABASE_URL	PostgreSQL connection string	postgres://postgres:postgres@postgres:5432/hardware_store?sslmode=disable
REDIS_URL	Redis connection string	redis://redis:6379
JWT_SECRET	JWT signing key (min 32 chars)	your-secret-key-must-be-long
ENCRYPTION_KEY	AES-256-GCM encryption key	64-character-hex-from-openssl
APP_URL	Frontend URL (CORS)	http://localhost:5173
Payment Providers
Provider	Variables	Purpose
M-Pesa	MPESA_CONSUMER_KEY, MPESA_CONSUMER_SECRET, MPESA_SHORTCODE, MPESA_PASSKEY, MPESA_CALLBACK_URL	Mobile money payments
Airtel Money	AIRTEL_CLIENT_ID, AIRTEL_CLIENT_SECRET	Airtel mobile payments
Card (Pesapal)	PESAPAL_CONSUMER_KEY, PESAPAL_CONSUMER_SECRET, PESAPAL_REDIRECT_URL, PESAPAL_CALLBACK_URL	Credit/debit card payments
Notifications
Provider	Variables	Purpose
Email	SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASSWORD, SMTP_FROM_NAME	Order confirmations, notifications
WhatsApp	GREENAPI_ID_INSTANCE, GREENAPI_API_TOKEN, GREENAPI_PHONE	Order updates via WhatsApp
Storage
Provider	Variables	Purpose
Cloudinary	CLOUDINARY_CLOUD_NAME, CLOUDINARY_API_KEY, CLOUDINARY_API_SECRET	Image uploads (POD photos)
🧪 Testing
Test Endpoints
The repository includes a test script:

bash
#### Make it executable
chmod +x test_endpoints.sh

#### Run tests
./test_endpoints.sh
Manual Testing with cURL
bash
#### Create a user
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123","first_name":"Test","last_name":"User"}'

#### Login
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}'

#### Get products
curl http://localhost:8080/api/v1/products

#### Get stores
curl http://localhost:8080/api/v1/stores
🐛 Troubleshooting
Migration Failures
Problem: "Dirty database version" error

Solution:

bash
#### Access database
docker-compose exec postgres psql -U postgres -d hardware_store

#### Reset migrations
TRUNCATE schema_migrations;
INSERT INTO schema_migrations (version, dirty) VALUES (15, false);

#### Restart API
docker-compose restart api
Port Already in Use
Problem: address already in use error

Solution: Change host ports in docker-compose.yml:

yaml
ports:
  - "5433:5432"  # Change host port
Container Restarting Loop
Problem: Container keeps restarting

Solution:

bash
#### Check logs for error
docker-compose logs api

#### Stop and restart with fresh database
docker-compose down -v
docker-compose up -d
Database Connection Issues
Problem: Can't connect to PostgreSQL

Solution:

bash
#### Check if database is running
docker-compose ps postgres

#### Check database logs
docker-compose logs postgres

#### Test connection
docker-compose exec postgres pg_isready -U postgres
🚢 Production Deployment
1. Environment Setup
bash
#### Use production environment
APP_ENV=production

#### Update callback URLs to your production domain
MPESA_CALLBACK_URL=https://your-domain.com/api/v1/payments/mpesa/callback
PESAPAL_CALLBACK_URL=https://your-domain.com/api/v1/payments/card/callback

#### Use production URLs
MPESA_BASE_URL=https://api.safaricom.co.ke
AIRTEL_BASE_URL=https://openapi.airtelkenya.com
2. Security
bash
#### Generate strong secrets
openssl rand -base64 32  # JWT_SECRET
openssl rand -hex 32     # ENCRYPTION_KEY

#### Never commit .env file to version control
echo ".env" >> .gitignore
3. Run in Production
bash
#### Use production compose file
docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d

#### Or with environment variables
APP_ENV=production docker-compose up -d
4. Configure SSL/HTTPS
For production, use a reverse proxy like Nginx or Traefik:

yaml
#### Example with Traefik
services:
  api:
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.api.rule=Host(`api.yourdomain.com`)"
      - "traefik.http.services.api.loadbalancer.server.port=8080"
📊 Monitoring
Health Check
bash
#### Simple health check
curl http://localhost:8080/health

#### Detailed metrics (if implemented)
curl http://localhost:8080/metrics
Logs
bash
#### Production logs with rotation
docker-compose logs --tail=100 -f api > logs.txt

#### Filter by severity
docker-compose logs api | grep "ERROR"
🗄️ Database Migrations
Migration Files
Migrations are stored in the migrations/ directory:

text
migrations/
├── 000_extensions.up.sql          # UUID extension
├── 001_create_users.up.sql        # Users table
├── 002_create_stores.up.sql       # Stores table
├── ...
└── 015_categories_subcategories.up.sql  # Latest migration
Running Migrations
bash
#### Auto-run (on container start)
docker-compose up -d

#### Manually
docker-compose exec api ./main migrate

#### Rollback (if supported)
docker-compose exec api ./main migrate down
🤝 Contributing
Fork the repository

Create your feature branch: git checkout -b feature/amazing-feature

Commit your changes: git commit -m 'Add amazing feature'

Push to the branch: git push origin feature/amazing-feature

Open a Pull Request

📄 License
This project is proprietary software.

📞 Support
Issues: GitHub Issues

Email: support@hardwarestore.com

🙏 Acknowledgments
Go - The programming language

PostgreSQL - Database

Redis - Caching

Docker - Containerization

Safaricom Daraja API - M-Pesa integration

Pesapal - Card payments

Green API - WhatsApp integration

Built with ❤️ for Wakulima Hardware

🎯 Quick Reference Card
bash
#### Development
docker-compose up -d
curl http://localhost:8080/health

#### Stop
docker-compose down

#### Reset everything
docker-compose down -v

#### Rebuild
docker-compose up -d --build

#### Logs
docker-compose logs -f api

#### Database
docker-compose exec postgres psql -U postgres -d hardware_store

#### Migrations
docker-compose exec api ./main migrate
