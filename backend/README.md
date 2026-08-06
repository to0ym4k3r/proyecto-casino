# Casino Backend API

Backend Go para la plataforma Casino Online. API REST con arquitectura hexagonal, autenticacion JWT, y tiempo real via PubNub.

## Stack

- **Go** 1.22+ con **chi** (router) y **pgx** (PostgreSQL driver)
- **PostgreSQL** 16 con pool de conexiones pgxpool
- **Docker** + docker-compose para desarrollo local
- **JWT** para autenticacion (Access 15min + Refresh 7d)
- **bcrypt** cost 12 para hashing de contrasenas

## Setup Local

### Prerequisitos
- Go 1.22+
- Docker y docker-compose
- Make (opcional, para comandos abreviados)

### Instalacion

```bash
# 1. Copiar variables de entorno
cp .env.example .env

# 2. Editar .env con tus credenciales de PostgreSQL
#    (o usar docker-compose para levantar PostgreSQL)

# 3. Levantar PostgreSQL
docker-compose up -d postgres

# 4. Ejecutar migraciones
make migrate

# 5. Iniciar el servidor
make run
```

El servidor arranca en `http://localhost:8080` por defecto.

### Variables de Entorno

| Variable | Descripcion | Default |
|----------|------------|---------|
| `DB_HOST` | Host de PostgreSQL | localhost |
| `DB_PORT` | Puerto de PostgreSQL | 5432 |
| `DB_USER` | Usuario de PostgreSQL | casino_user |
| `DB_PASSWORD` | Contrasena de PostgreSQL | casino_pass |
| `DB_NAME` | Nombre de la base de datos | casino |
| `DATABASE_URL` | DSN completo usado por `make migrate` (si se omite, se construye desde `DB_*`) | vacio |
| `DB_SSLMODE` | Modo SSL de PostgreSQL | disable |
| `JWT_SECRET` | Secreto para firmar JWT (cambiar en produccion!) | change-me-in-production |
| `JWT_ACCESS_EXPIRY` | Duracion del access token (minutos) | 15 |
| `JWT_REFRESH_EXPIRY` | Duracion del refresh token (minutos) | 10080 (7 dias) |
| `PORT` | Puerto del servidor HTTP | 8080 |
| `PUBNUB_PUBLISH_KEY` | Clave pub de PubNub | -- |
| `PUBNUB_SUBSCRIBE_KEY` | Clave sub de PubNub | -- |
| `PUBNUB_SECRET_KEY` | Secreto de PubNub | -- |
| `SOLANA_RPC_URL` | URL del RPC de Solana | -- |
| `SOLANA_WALLET_PRIVATE_KEY` | Clave privada del wallet (nunca en codigo!) | -- |
| `CORS_ALLOWED_ORIGINS` | Origenes CORS permitidos | http://localhost:3000 |

## Endpoints API

### Autenticacion

| Metodo | Ruta | Auth | Descripcion |
|--------|------|------|------------|
| `POST` | `/api/v1/auth/register` | No | Crear cuenta nueva |
| `POST` | `/api/v1/auth/login` | No | Iniciar sesion, retorna tokens JWT |
| `POST` | `/api/v1/auth/refresh` | No | Renovar access token usando refresh token |

#### POST /api/v1/auth/register
```json
// Request
{
  "email": "usuario@email.com",
  "username": "jugador1",
  "password": "Segura123!"
}

// Response 201
{
  "success": true,
  "data": {
    "id": "uuid",
    "email": "usuario@email.com",
    "username": "jugador1",
    "created_at": "2025-01-01T00:00:00Z"
  }
}
```

#### POST /api/v1/auth/login
```json
// Request
{
  "email": "usuario@email.com",
  "password": "Segura123!"
}

// Response 200
{
  "success": true,
  "data": {
    "access_token": "eyJhbGciOiJIUzI1...",
    "refresh_token": "eyJhbGciOiJIUzI1...",
    "expires_in": 900
  }
}
```

#### POST /api/v1/auth/refresh
```json
// Request
{
  "refresh_token": "eyJhbGciOiJIUzI1..."
}

// Response 200
{
  "success": true,
  "data": {
    "access_token": "eyJhbGciOiJIUzI1...",
    "refresh_token": "eyJhbGciOiJIUzI1...",
    "expires_in": 900
  }
}
```

### Juegos

| Metodo | Ruta | Auth | Descripcion |
|--------|------|------|------------|
| `GET` | `/api/v1/games/` | No | Listar todos los juegos disponibles |
| `GET` | `/api/v1/games/{gameID}` | No | Obtener detalle de un juego |
| `POST` | `/api/v1/games/start` | Si | Iniciar una sesion de juego |

### Wallet

| Metodo | Ruta | Auth | Descripcion |
|--------|------|------|------------|
| `GET` | `/api/v1/wallet/balance` | Si | Ver balance actual |
| `POST` | `/api/v1/wallet/deposit` | Si | Depositar fondos |
| `POST` | `/api/v1/wallet/withdraw` | Si | Retirar fondos |

## Autenticacion

Los endpoints protegidos requieren un header `Authorization`:

```
Authorization: Bearer <access_token>
```

Los tokens se obtienen en `/api/v1/auth/login` y se renuevan con `/api/v1/auth/refresh`.

**Flujo de tokens:**
1. Login -> access_token (15min) + refresh_token (7d)
2. Usar access_token en requests protegidos
3. Cuando expire, enviar refresh_token a `/api/v1/auth/refresh`
4. Obtener nuevo par de tokens

## Seguridad

| Medida | Implementacion |
|--------|---------------|
| SQL Injection | Queries parametrizados ($1, $2...) |
| Password Hashing | bcrypt cost 12 |
| JWT | Access 15min, Refresh 7d |
| Rate Limiting | Token bucket: auth 5/min, general 100/min |
| Headers | CSP, HSTS, X-Frame-Options DENY |
| CORS | Configurable via CORS_ALLOWED_ORIGINS |

## Docker

```bash
# Full stack (PostgreSQL + Backend)
docker-compose up -d --build

# Solo PostgreSQL
docker-compose up -d postgres

# Ver logs
docker-compose logs -f backend
```

## Desarrollo

```bash
make test          # Ejecutar tests
make lint          # Linter (golangci-lint)
make build         # Compilar binario
make run           # Ejecutar directamente
make migrate       # Aplicar migraciones SQL pendientes
make docker-build  # Build imagen Docker
```

## Arquitectura

Ver [docs/architecture.md](docs/architecture.md) para la documentacion completa de la arquitectura hexagonal.
