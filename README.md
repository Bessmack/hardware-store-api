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





# 📚 API Routes Documentation

All API endpoints are prefixed with `/api/v1`. The API uses **JWT authentication** and **role-based access control**.

---

## 🔐 Authentication

Most endpoints require authentication. Include the JWT token in the `Authorization` header:

```http
Authorization: Bearer <your-jwt-token>
```

---

## 👥 User Roles

| Role         | Description                                                                 |
| ------------ | --------------------------------------------------------------------------- |
| `customer`   | Regular customers — can browse, order, and manage their profile             |
| `cashier`    | Store staff — can manage orders and inventory for their assigned store      |
| `admin`      | Store administrators — full access to their store's operations              |
| `superadmin` | Platform administrators — full access to all stores and system settings     |

---

## 📍 Public Endpoints

### Health Check

```http
GET /api/v1/health
```

**Response:**

```json
{ "status": "ok" }
```

### Payment Methods

```http
GET /api/v1/payments/methods
```

Returns the list of available payment methods (MPesa, Airtel, Card).

### Stores

| Method | Endpoint                              | Description                          |
| ------ | ------------------------------------- | ------------------------------------ |
| GET    | `/api/v1/stores`                      | List all active stores               |
| GET    | `/api/v1/stores/{storeID}`            | Get store details                    |
| GET    | `/api/v1/stores/{storeID}/products`   | Get products for a specific store    |

### Products

| Method | Endpoint                        | Description         |
| ------ | ------------------------------- | ------------------- |
| GET    | `/api/v1/products/{productID}`  | Get product details |

### Geolocation

| Method | Endpoint                      | Description                              |
| ------ | ----------------------------- | ---------------------------------------- |
| POST   | `/api/v1/geo/location`        | Save user location (optional auth)       |
| GET    | `/api/v1/geo/autocomplete`    | Address autocomplete                     |
| GET    | `/api/v1/geo/geocode`         | Geocode address to coordinates           |

---

## 🔑 Auth Endpoints

| Method | Endpoint                | Description         | Rate Limit |
| ------ | ----------------------- | ------------------- | ---------- |
| POST   | `/api/v1/auth/register` | Register new user   | Tight      |
| POST   | `/api/v1/auth/login`    | Login user          | Tight      |
| POST   | `/api/v1/auth/refresh`  | Refresh JWT token   | Standard   |
| POST   | `/api/v1/auth/logout`   | Logout user         | Standard   |

**Registration Request:**

```json
{
  "email": "user@example.com",
  "phone": "254712345678",
  "password": "password123",
  "first_name": "John",
  "last_name": "Doe"
}
```

**Login Request:**

```json
{
  "email": "user@example.com",
  "password": "password123"
}
```

**Login Response:**

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
  "user": {
    "id": "uuid",
    "email": "user@example.com",
    "first_name": "John",
    "last_name": "Doe",
    "role": "customer"
  }
}
```

---

## 👤 Customer Endpoints

> **Requires:** `customer`, `cashier`, `admin`, or `superadmin` role

### User Profile

| Method | Endpoint           | Description                  |
| ------ | ------------------ | ---------------------------- |
| GET    | `/api/v1/users/me` | Get current user profile     |
| PUT    | `/api/v1/users/me` | Update current user profile  |

### Cart

| Method | Endpoint                          | Description                              |
| ------ | --------------------------------- | ---------------------------------------- |
| GET    | `/api/v1/cart`                    | Get current cart                         |
| POST   | `/api/v1/cart/items`              | Add item to cart                         |
| PUT    | `/api/v1/cart/items/{itemID}`     | Update item quantity                     |
| DELETE | `/api/v1/cart/items/{itemID}`     | Remove item from cart                    |
| GET    | `/api/v1/cart/validate`           | Validate cart (check prices, stock)      |

**Add to Cart Request:**

```json
{
  "product_id": "uuid",
  "store_id": "uuid",
  "quantity": 2
}
```

### Wishlist

| Method | Endpoint                                 | Description                      |
| ------ | ---------------------------------------- | -------------------------------- |
| GET    | `/api/v1/wishlist`                       | Get user's wishlist              |
| POST   | `/api/v1/wishlist/items`                 | Add product to wishlist          |
| DELETE | `/api/v1/wishlist/items/{productID}`     | Remove product from wishlist     |

**Add to Wishlist Request:**

```json
{ "product_id": "uuid" }
```

### Delivery

| Method | Endpoint                   | Description         |
| ------ | -------------------------- | ------------------- |
| GET    | `/api/v1/delivery/quote`   | Get delivery quote  |

**Delivery Quote Request:**

```json
{
  "store_id": "uuid",
  "delivery_address": {
    "latitude": -1.2921,
    "longitude": 36.8219
  },
  "cart_items": [
    { "product_id": "uuid", "quantity": 2 }
  ]
}
```

### Orders

| Method | Endpoint                              | Description                          |
| ------ | ------------------------------------- | ------------------------------------ |
| POST   | `/api/v1/orders`                      | Place new order                      |
| GET    | `/api/v1/orders`                      | List user's orders                   |
| GET    | `/api/v1/orders/{orderID}`            | Get specific order                   |
| GET    | `/api/v1/orders/{orderID}/track`      | Track order status                   |
| DELETE | `/api/v1/orders/{orderID}`            | Cancel order                         |
| POST   | `/api/v1/orders/{orderID}/dispute`    | Raise a dispute (after delivery)     |

**Place Order Request:**

```json
{
  "store_id": "uuid",
  "delivery_type": "delivery",
  "delivery_address": {
    "address_text": "123 Main St, Nairobi",
    "latitude": -1.2921,
    "longitude": 36.8219
  },
  "payment_method": "mpesa",
  "payment_provider": "mpesa"
}
```

---

## 🏪 Staff Endpoints

> **Requires:** `cashier`, `admin`, or `superadmin` role + store scope

### Store Operations

| Method | Endpoint                                    | Description                       |
| ------ | ------------------------------------------- | --------------------------------- |
| GET    | `/api/v1/store/orders`                      | List orders for assigned store    |
| GET    | `/api/v1/store/orders/{orderID}`            | Get order details                 |
| PUT    | `/api/v1/store/orders/{orderID}/status`     | Update order status               |

**Update Order Status Request:**

```json
{
  "status": "confirmed",
  "note": "Order confirmed, preparing for dispatch"
}
```

### Inventory Management

| Method | Endpoint                                  | Description           |
| ------ | ----------------------------------------- | --------------------- |
| GET    | `/api/v1/store/inventory`                 | List store inventory  |
| PUT    | `/api/v1/store/inventory/{productID}`     | Update inventory      |

**Update Inventory Request:**

```json
{
  "price": 1500.00,
  "stock_quantity": 100,
  "is_available": true
}
```

### Product Management

| Method | Endpoint                                 | Description          |
| ------ | ---------------------------------------- | -------------------- |
| POST   | `/api/v1/store/products`                 | Create new product   |
| PUT    | `/api/v1/store/products/{productID}`     | Update product       |
| DELETE | `/api/v1/store/products/{productID}`     | Deactivate product   |

**Create Product Request:**

```json
{
  "name": "Hammer 16oz",
  "description": "Steel hammer with wooden handle",
  "category": "tools",
  "weight_kg": 0.5,
  "images": ["public_id_1", "public_id_2"],
  "constraint_type": "weight"
}
```

### Proof of Delivery (POD)

| Method | Endpoint                                     | Description                  |
| ------ | -------------------------------------------- | ---------------------------- |
| POST   | `/api/v1/store/pod/submit`                   | Submit proof of delivery     |
| GET    | `/api/v1/store/orders/{orderID}/pod`         | Get POD details              |
| GET    | `/api/v1/store/orders/{orderID}/dispute`     | Get dispute details          |

**Submit POD Request:**

```json
{
  "order_id": "uuid",
  "otp_code": "123456",
  "photo_public_id": "delivery-photos/order-abc123",
  "latitude": -1.2921,
  "longitude": 36.8219
}
```

### Delivery Rates

| Method | Endpoint                                              | Description                    |
| ------ | ----------------------------------------------------- | ------------------------------ |
| GET    | `/api/v1/store/delivery/rates`                        | Get store delivery rates       |
| PUT    | `/api/v1/store/delivery/rates`                        | Update store delivery rates    |
| DELETE | `/api/v1/store/delivery/rates/{vehicleType}`          | Delete store rate override     |

**Update Delivery Rates Request:**

```json
{
  "vehicle_type": "bike",
  "base_fee": 60.00,
  "per_km": 60.00,
  "max_weight_kg": 130.00,
  "max_radius_km": 20.00
}
```

### Dispute Resolution

| Method | Endpoint                                          | Description         |
| ------ | ------------------------------------------------- | ------------------- |
| PUT    | `/api/v1/store/disputes/{disputeID}/resolve`      | Resolve a dispute   |

**Resolve Dispute Request:**

```json
{
  "status": "resolved",
  "notes": "Issue resolved with customer"
}
```

### Store Reports

| Method | Endpoint                | Description            |
| ------ | ----------------------- | ---------------------- |
| GET    | `/api/v1/store/report`  | Generate store report  |

---

## 👑 Superadmin Endpoints

> **Requires:** `superadmin` role only

### Store Management

| Method | Endpoint                                      | Description                                  |
| ------ | --------------------------------------------- | -------------------------------------------- |
| POST   | `/api/v1/stores`                              | Create new store                             |
| PUT    | `/api/v1/stores/{storeID}`                    | Update store                                 |
| PUT    | `/api/v1/stores/{storeID}/credentials`        | Update M-Pesa credentials                    |
| PUT    | `/api/v1/stores/{storeID}/deactivate`         | Deactivate store                             |
| PUT    | `/api/v1/stores/{storeID}/reactivate`         | Reactivate store                             |
| GET    | `/api/v1/stores/all`                          | List all stores (including inactive)         |

**Create Store Request:**

```json
{
  "name": "Nairobi Branch",
  "address": "123 Main St",
  "county": "Nairobi",
  "latitude": -1.2921,
  "longitude": 36.8219,
  "phone": "254712345678",
  "email": "nairobi@hardware.store",
  "mpesa_paybill": "522522",
  "mpesa_shortcode": "174379",
  "mpesa_passkey": "passkey_here"
}
```

**Update Store Credentials Request:**

```json
{
  "mpesa_consumer_key": "consumer_key",
  "mpesa_consumer_secret": "consumer_secret"
}
```

### Global Delivery Rates

| Method | Endpoint                              | Description                            |
| ------ | ------------------------------------- | -------------------------------------- |
| PUT    | `/api/v1/delivery/rates/global`       | Update global default delivery rates   |

**Update Global Rates Request:**

```json
{
  "vehicle_type": "pickup",
  "base_fee": 500.00,
  "per_km": 380.00,
  "max_weight_kg": 2000.00,
  "max_radius_km": 120.00
}
```

### Global Reports

| Method | Endpoint                  | Description                       |
| ------ | ------------------------- | --------------------------------- |
| GET    | `/api/v1/reports/global`  | Generate global platform report   |

### User Management

| Method | Endpoint                    | Description                  |
| ------ | --------------------------- | ---------------------------- |
| GET    | `/api/v1/users`             | List all admins and staff    |
| GET    | `/api/v1/users/{userID}`    | Get user details             |
| PUT    | `/api/v1/users/{userID}`    | Deactivate user *(TODO)*     |

---

## 💳 Payment Callbacks (Webhooks)

> These endpoints are called by payment providers and **do not require authentication**.

| Method | Endpoint                                              | Description                                |
| ------ | ----------------------------------------------------- | ------------------------------------------ |
| POST   | `/api/v1/payments/mpesa/callback/{storeID}`           | M-Pesa payment callback (IP allowlist)     |
| POST   | `/api/v1/payments/airtel/callback/{storeID}`          | Airtel payment callback (IP allowlist)     |
| POST   | `/api/v1/payments/card/callback`                      | Pesapal card payment callback              |

---

## 📊 Rate Limits

Different endpoints have different rate limits to prevent abuse:

| Endpoint                  | Rate Limit                |
| ------------------------- | ------------------------- |
| `/api/v1/auth/register`   | Tight (5 per minute)      |
| `/api/v1/auth/login`      | Tight (5 per minute)      |
| `/api/v1/auth/refresh`    | Standard (30 per minute)  |
| `/api/v1/geo/*`           | Tight (10 per minute/IP)  |
| All other endpoints       | Standard (60 per minute/IP) |

---

## 🔒 Security Notes

- **Authentication:** JWT tokens are required for all protected endpoints. Tokens expire after 30 minutes (configurable via `JWT_ACCESS_EXPIRY_MINUTES`).
- **Refresh Tokens:** Use the `/refresh` endpoint to get a new access token. Refresh tokens expire after 7 days.
- **IP Allowlisting:** M-Pesa and Airtel callbacks are protected by IP allowlisting to ensure only provider servers can call them.
- **Encryption:** Sensitive fields (M-Pesa credentials) are encrypted using **AES-256-GCM** before storing in the database.
- **Role-Based Access:** Routes are protected by role-based access control. Customers cannot access staff routes, and staff cannot access superadmin routes.
- **Store Scoping:** Staff members are automatically scoped to their assigned store. They can only access data for their store.

---

## 💡 Common Use Cases

### 1. Complete Customer Checkout Flow

```bash
# 1. Register/Login
POST /api/v1/auth/register
POST /api/v1/auth/login

# 2. Get delivery quote
GET /api/v1/delivery/quote?store_id=...&lat=...&lng=...

# 3. Place order
POST /api/v1/orders

# 4. Track order
GET /api/v1/orders/{orderID}/track

# 5. Raise dispute (after delivery)
POST /api/v1/orders/{orderID}/dispute
```

### 2. Staff Order Fulfillment Flow

```bash
# 1. View store orders
GET /api/v1/store/orders

# 2. Update order status
PUT /api/v1/store/orders/{orderID}/status

# 3. Submit POD
POST /api/v1/store/pod/submit

# 4. Handle disputes
GET /api/v1/store/orders/{orderID}/dispute
PUT /api/v1/store/disputes/{disputeID}/resolve
```

### 3. Superadmin Store Setup

```bash
# 1. Create store
POST /api/v1/stores

# 2. Configure M-Pesa credentials
PUT /api/v1/stores/{storeID}/credentials

# 3. Set delivery rates
PUT /api/v1/store/delivery/rates
PUT /api/v1/delivery/rates/global

# 4. Manage staff
GET /api/v1/users
PUT /api/v1/users/{userID}
```

---

## 🧪 Testing Endpoints

Use the provided test script:

```bash
./test_endpoints.sh
```

Or test individual endpoints with **cURL**:

```bash
# Health check
curl http://localhost:8080/api/v1/health

# Get stores
curl http://localhost:8080/api/v1/stores

# Login
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}'
```

> **Note:** All endpoints return JSON responses. Error responses follow this format:

```json
{
  "error": "Error message description",
  "status": 400
}
```
