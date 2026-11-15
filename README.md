# Dubai Properties

A three-tier Dubai real-estate listing application.

## Stack
- Frontend: React 18, Vite, React Router, plain CSS
- API: Go (`net/http`), MongoDB Go Driver, JWT, bcrypt
- Database: MongoDB; property images stored in MongoDB GridFS
- Local runtime: Docker Compose

## Features
- Register/login with bcrypt password hashing and JWT authentication
- Public property browse, search, filters and property details
- Authenticated property create/update/delete
- Admin role can manage any listing; owners can manage their own
- Multi-image upload to GridFS and image streaming endpoint
- Responsive real-estate UI with demo seed listings
- API unit tests for health and validation helpers

## Run locally
Requirements: Docker Desktop / Docker Engine with Compose.

```bash
cp .env.example .env
docker compose up --build
```

Open http://localhost:5173

The API is at http://localhost:8081/api and health check at http://localhost:8081/health.
MongoDB is exposed on localhost:27018 for local development only.

## First admin
Register normally, then promote the user in MongoDB:

```bash
docker compose exec mongo mongosh -u "$MONGO_ROOT_USERNAME" -p "$MONGO_ROOT_PASSWORD" --authenticationDatabase admin
```
In mongosh:
```javascript
use dubai_properties
db.users.updateOne({email: "your-email@example.com"}, {$set: {role: "admin"}})
```
Sign out and sign in again to receive an admin JWT.

## API overview
- `POST /api/auth/register`
- `POST /api/auth/login`
- `GET /api/auth/me`
- `GET /api/properties`
- `GET /api/properties/:id`
- `POST /api/properties` (auth; multipart form, images field)
- `PUT /api/properties/:id` (owner/admin)
- `DELETE /api/properties/:id` (owner/admin)
- `GET /api/images/:id` (GridFS image stream)

Property fields: `title`, `description`, `price`, `purpose` (`sale`/`rent`), `type`, `location`, `bedrooms`, `bathrooms`, `area`, `features` (comma-separated), `images` (files).

## Notes before production
This is a portfolio starter, not a production-certified real-estate platform. Before public deployment add email verification, password reset, rate limiting, CSRF strategy if using cookies, audit logging, malware/image-content validation, image size limits at ingress, backups, MongoDB TLS/private networking, refresh-token rotation, structured logs, metrics, and integration/E2E tests. Keep secrets in Secret Manager, not Git. Use HTTPS and a managed MongoDB service or secured private database deployment.
