# Casino Online

> Plataforma de casino online con app Android nativa, backend Go de alta performance, tiempo real con PubNub y pagos crypto Solana.

## Descripcion

Casino Online es una plataforma de juego completa que combina una experiencia premium con un backend robusto y disenada para ofrecer sorteos, slots, ruleta, cartas y apuestas en tiempo real.

## Stack Tecnologico

| Capa | Tecnologia | Version |
|------|-----------|---------|
| Frontend Movil | Kotlin + Jetpack Compose + Material Design 3 | SDK 35 |
| Backend API | Go + chi (router) + pgx (driver PostgreSQL) | Go 1.25+ |
| Base de Datos | PostgreSQL | 16 |
| Tiempo Real | PubNub | -- |
| Crypto Pagos | Solana | -- |
| Auth | JWT (Access 15min + Refresh 7d) + bcrypt | cost 12 |
| Container | Docker + docker-compose | -- |
| DI (Android) | Hilt | -- |
| DI (Go) | Manual constructor injection | -- |

## Arquitectura

El proyecto sigue **Arquitectura Hexagonal** (Ports & Adapters):

```
+--------------------------------------------------+
|                  CASINO ONLINE                    |
+---------------------+----------------------------+
|   ANDROID APP       |      GO BACKEND            |
|   (Kotlin/Compose)  |   (Hexagonal/Ports)        |
|                     |                            |
|  +--------------+   |  +--------------------+    |
|  | UI Screens   |   |  | Handlers (adapters)|    |
|  | Navigation   |   |  |                    |    |
|  | ViewModel    |   |  | Services (use cas.)|    |
|  | DI (Hilt)    |   |  |                    |    |
|  | Theme        |   |  | Repository (ports) |    |
|  +------+-------+   |  |                    |    |
|         |           |  | PostgreSQL (adapter)|    |
|         |           |  +--------------------+    |
|         |           |                            |
|         +---------->|--- REST API ------> PubNub |
|                     |                            |
|                     |  +--------------------+    |
|                     |  | Middlewares:        |    |
|                     |  | - Rate Limiting     |    |
|                     |  | - JWT Auth          |    |
|                     |  | - Security Headers  |    |
|                     |  | - CORS              |    |
|                     |  | - Logging           |    |
|                     |  +--------------------+    |
+---------------------+----------------------------+
```

Ver [backend/docs/architecture.md](backend/docs/architecture.md) para detalles de la arquitectura hexagonal.

## Prerequisitos

- **Go** 1.25 o superior (`backend/go.mod` fija `go 1.25.0`)
- **Java JDK** 17
- **Android Studio** (con SDK 35, build-tools 35)
- **Docker** y **docker-compose**
- **PostgreSQL** 16 (se levanta via Docker)

## Instalacion Rapida

### 1. Clonar el repositorio
```bash
git clone https://github.com/to0ym4k3r/proyecto-casino.git
cd proyecto-casino
```

### 2. Configurar variables de entorno
```bash
cp backend/.env.example backend/.env
# Editar backend/.env con tus credenciales
```

### 3. Levantar PostgreSQL
```bash
cd backend
docker-compose up -d postgres
```

### 4. Ejecutar migraciones
```bash
make migrate
```

### 5. Iniciar el backend
```bash
make run
# El servidor arranca en http://localhost:8080
```

### 6. Abrir la app Android
```bash
# Abrir android/ en Android Studio
# Sincronizar Gradle
# Ejecutar en emulador o dispositivo
```

## Estructura del Proyecto

```
proyecto-casino/
+-- DESIGN.md                    <- Design system (tokens, colores, tipografia)
+-- README.md                    <- Este archivo
+-- SECURITY.md                  <- Politica de seguridad y gambling responsable
+-- CONTRIBUTING.md              <- Guia de contribucion
|
+-- android/                     <- App Android nativa
|   +-- app/src/main/java/com/casino/app/
|   |   +-- CasinoApp.kt        <- Application class (Hilt)
|   |   +-- MainActivity.kt     <- Activity principal
|   |   +-- di/                  <- Inyeccion de dependencias
|   |   +-- ui/
|   |       +-- navigation/      <- Navegacion entre pantallas
|   |       +-- screens/         <- Pantallas (Lobby, Game, Wallet, Profile)
|   |       +-- theme/           <- Tema Material3
|   +-- app/build.gradle.kts     <- Configuracion de compilacion
|   +-- build.gradle.kts         <- Modulo app
|
+-- backend/                     <- API Go
|   +-- cmd/server/main.go       <- Punto de entrada
|   +-- internal/
|   |   +-- config/              <- Configuracion por entorno
|   |   +-- domain/              <- Modelos de negocio (el core)
|   |   +-- handler/             <- HTTP handlers (adapters de entrada)
|   |   +-- middleware/          <- Auth, rate limit, security, CORS, logging
|   |   +-- repository/          <- Interfaces + implementacion PostgreSQL
|   |   +-- service/             <- Logica de negocio (use cases)
|   +-- pkg/
|   |   +-- logger/              <- Logging estructurado
|   |   +-- response/            <- Helpers de respuesta JSON
|   +-- migrations/              <- Esquemas SQL
|   +-- docs/                    <- Documentacion tecnica
|   +-- Dockerfile               <- Imagen Docker del backend
|   +-- docker-compose.yml       <- PostgreSQL + backend
|   +-- Makefile                 <- Comandos de desarrollo
|   +-- .env.example             <- Variables de entorno de ejemplo
|
+-- .agents/skills/              <- 12 skills de AI para desarrollo
```

## Seguridad

El backend implementa 7 capas de seguridad desde el primer dia:

| # | Capa | Detalle |
|---|------|---------|
| 1 | SQL Injection | Queries parametrizados ($1, $2...) - cero concatenacion |
| 2 | Rate Limiting | Token bucket: auth 5/min, general 100/min |
| 3 | Autenticacion | JWT Access (15min) + Refresh (7d) |
| 4 | Password Hashing | bcrypt cost factor 12 |
| 5 | Headers de Seguridad | CSP, HSTS, X-Frame-Options DENY |
| 6 | CORS | Origenes configurables, no abierto |
| 7 | Gambling Responsable | Ver [SECURITY.md](SECURITY.md) |

Ver [SECURITY.md](SECURITY.md) para la politica completa y checklist de auditoria.

## Endpoints API

Ver [backend/README.md](backend/README.md) para la referencia completa de la API.

## Desarrollo

### Backend
```bash
cd backend
make test          # Ejecutar tests
make lint          # Linter
make build         # Compilar
```

### Android
```bash
cd android
./gradlew assembleDebug    # Build debug
./gradlew test             # Tests unitarios
./gradlew lint             # Lint check
```

## Contribuir

Ver [CONTRIBUTING.md](CONTRIBUTING.md) para la guia completa de contribucion.

## Licencia

MIT License - Ver [LICENSE](LICENSE) para detalles.

---

**Desarrollado con carino para gaming seguro y responsable**
