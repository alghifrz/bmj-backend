# Buana Medika Jaya Backend

## Project

Buana Medika Jaya Backend

```text
Next.js frontend
        ↓
Go + Gin backend
        ↓
Supabase PostgreSQL
        +
Supabase Storage
```

The API serves the Buana Medika Jaya website. PostgreSQL and file storage are hosted by Supabase. This repository currently provides configuration, routing, health checks, SQL migrations, and the initial database schema.

## Stack

* Go
* Gin
* Supabase PostgreSQL
* pgx
* Supabase Storage
* Docker

## Required environment variables

```text
DATABASE_URL
SUPABASE_URL
SUPABASE_SERVICE_ROLE_KEY
SUPABASE_STORAGE_BUCKET
JWT_SECRET
```

`DATABASE_URL` is the Supabase PostgreSQL connection string. `SUPABASE_URL` is the project URL. `SUPABASE_SERVICE_ROLE_KEY` is a server-side secret: do not commit it, log it, or expose it to the frontend. `SUPABASE_STORAGE_BUCKET` selects the Storage bucket for product images. Image upload is not implemented yet.

Copy the example file and fill in the Supabase values locally:

```bash
cp .env.example .env
```

`CORS_ORIGIN` is the frontend origin allowed to call the API from a browser. Separate extra origins with commas. Do not set `CORS_ORIGIN=*` when `APP_ENV=production`.

## Local Development

### Run the backend

```bash
go run ./cmd/api
```

The server listens on `APP_PORT` (default `8080`). It still starts when Supabase is unreachable. `/health` and `/api/v1/health` do not query the database.

### Migration

Migrations run against the database in `DATABASE_URL`.

```bash
go run ./cmd/migrate up
go run ./cmd/migrate down
go run ./cmd/migrate version
```

`down` rolls back the latest applied migration.

### Run tests

```bash
go test ./...
```

Tests do not require Supabase credentials.

### Run with Docker

```bash
docker build -t bmj-backend .
docker run --rm -p 8080:8080 --env-file .env bmj-backend
```

Or:

```bash
docker compose up --build
```

Compose starts the API only. It does not run a local PostgreSQL service. The container listens on port `8080`.

## Public API

Published catalog data only. Responses use `{"data": ..., "message": "Success"}`.

```text
GET /api/v1/products
GET /api/v1/products/:slug
GET /api/v1/categories
GET /api/v1/categories/:slug
GET /api/v1/reviews
GET /api/v1/store
```

Product list query parameters:

```text
page
limit
search
category
brand
availability
featured
sort
```

`sort` accepts `featured`, `newest`, `name_asc`, `name_desc`, `price_asc`, and `price_desc`. Price sorting places products without a public price last. Hidden prices are not returned.

## Admin authentication

Set `JWT_SECRET` in `.env` to a random value of at least 32 characters. Do not commit it.

Create the first admin. `ADMIN_PASSWORD` is read for that command only. Do not store it in `.env`.

```bash
# PowerShell
$env:ADMIN_PASSWORD="choose-a-password"
go run ./cmd/admin create -name "Owner" -email "you@example.com"
Remove-Item Env:ADMIN_PASSWORD
```

```text
POST /api/v1/admin/login
GET  /api/v1/admin/me
POST /api/v1/admin/logout
```

Send `Authorization: Bearer <token>` on every other admin route. The token expires after 12 hours. Logout tells the client to discard the token. Public catalog routes stay open and still return published records only. Hidden product prices stay hidden on public routes.

## Admin catalog

```text
GET    /api/v1/admin/products
POST   /api/v1/admin/products
PATCH  /api/v1/admin/products/:id
DELETE /api/v1/admin/products/:id

GET    /api/v1/admin/categories
POST   /api/v1/admin/categories
PATCH  /api/v1/admin/categories/:id
DELETE /api/v1/admin/categories/:id

GET    /api/v1/admin/reviews
POST   /api/v1/admin/reviews
PATCH  /api/v1/admin/reviews/:id
DELETE /api/v1/admin/reviews/:id

GET    /api/v1/admin/store
PATCH  /api/v1/admin/store
POST   /api/v1/admin/store/locations
PATCH  /api/v1/admin/store/locations/:id
DELETE /api/v1/admin/store/locations/:id
```

Create returns `201`. Update, delete, and list return `200`. A patch body must change at least one field. Sending JSON `null` clears an optional field. Product and category slugs use lowercase letters, numbers, and hyphens. A duplicate slug returns `409`. Deleting a category that still has products returns `409`.

Admin product responses include the stored price, including prices hidden from the public catalog. Image upload is not part of these routes.

## Health endpoints

```text
GET /health
GET /api/v1/health
GET /api/v1/health/db
```

`/health` and `/api/v1/health` return `200` `{"status":"ok"}`.

`/api/v1/health/db` returns `200` when Supabase PostgreSQL is reachable, and `503` when it is not.
