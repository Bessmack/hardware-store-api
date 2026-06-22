## hardware-store-api

# FIRST TIME SETUP???

## 1.) Clone your repo
git clone <[repo-url](https://github.com/Bessmack/hardware-store-api)>
cd hardware-store-api

### Copy environment file
cp .env.example .env

### Edit .env with your credentials
nano .env  # or vim, or use your editor

## 2.) Start everything:

-- Build and start all services
docker-compose up -d

-- Check if everything is running
docker-compose ps

-- View logs
docker-compose logs -f api

-- Run migrations (if not auto-run)
docker-compose exec api ./main migrate
