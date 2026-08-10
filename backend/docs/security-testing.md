# Pruebas de Defensa — Backend Casino Online

Documento de referencia de la suite de pruebas de seguridad del backend.
Describe qué se prueba, cómo se ejecuta, qué resultados arroja y qué hallazgos
de la auditoría están confirmados o mitigados.

## 1. Alcance y principio

Esta suite NO modifica código de producción. Solo añade archivos `_test.go` y
documentación. Su propósito es:

1. Comprobar que los mecanismos de defensa existentes responden ante vectores
   de ataque reales (falsificación JWT, inyección SQL, spoofing de rate limit,
   IDOR, validación de entrada, cabeceras de seguridad).
2. Dejar constancia permanente (tests "tripwire") de que los hallazgos de la
   auditoría siguen presentes o han sido mitigados, leyendo el código fuente de
   producción.
3. Escanear el repositorio en busca de backdoors y secretos reales.

## 2. Cómo ejecutar la suite

```powershell
# Desde el directorio backend/

# 1. Análisis estático del compilador
go vet ./...

# 2. Suite de seguridad — unidad (no requiere base de datos)
go test ./internal/security/ -v -timeout 240s

# 3. Suite de seguridad — con base de datos real (postgres descartable)
#    Levantar primero un contenedor, por ejemplo:
#    docker run -d --name casino-pg16-test -e POSTGRES_USER=casino_user ^
#      -e POSTGRES_PASSWORD=casino_pass -e POSTGRES_DB=casino ^
#      -p 5433:5432 postgres:16-alpine
$env:TEST_DATABASE_URL = "postgres://casino_user:casino_pass@localhost:5433/casino?sslmode=disable"
go test ./internal/security/ -v -timeout 240s

# 4. Escaneo de vulnerabilidades de dependencias
go run golang.org/x/vuln/cmd/govulncheck@latest ./...

# 5. Cobertura que la suite aporta al módulo completo
go test ./internal/security/ -skip "^TestBackdoorScan$" `
  -coverpkg="casino-backend/..." -coverprofile=coverage.out -covermode=set
go tool cover "-func=coverage.out"
```

> **Nota:** `go test -race` no se puede ejecutar en esta máquina de desarrollo:
> requiere CGO y no hay compilador C (`gcc`) en `%PATH%`. Coincide con el
> comentario de `.github/workflows/ci.yml` ("CGO is disabled on the Windows dev
> box"). El job de CI es el lugar indicado para correr `-race`.

## 3. Estructura de la suite (paquete `internal/security`)

| Archivo | Qué cubre |
|---|---|
| `helpers_test.go` | Fakes en memoria (`memUserRepo`, `memWalletRepo`, `memGameRepo`), constructor de handlers/router reales, emisor de tokens, helper HTTP (`doJSON`). |
| `jwt_test.go` | Falsificación JWT (9 vectores), subject all-zero, ausencia de `iss`/`aud`/`jti`. |
| `authz_middleware_test.go` | Middleware de autenticación (7 casos) y límite de IDOR por claims. |
| `ratelimit_test.go` | Capacidad de ráfaga (429), bypass por `X-Forwarded-For`, compartición de bucket entre clientes que usan el mismo XFF. |
| `input_validation_test.go` | Validación de registro y montos de wallet; cuerpos sobredimensionados y ausencia de `http.MaxBytesReader`. |
| `headers_test.go` | Cabeceras de seguridad directas y a través del router completo. |
| `backdoor_scan_test.go` | Escaneo estático del repo: exec, dial, HTTP client, rutas de admin/debug/pprof, claves privadas, secretos y JWT por defecto. |
| `static_findings_test.go` | Tripwires que afirman los hallazgos 2, 4, 5, 6, 7 y 14 leyendo la fuente. |
| `sqli_http_test.go` | Payloads de inyección SQL tratados como datos a través de los handlers reales. |
| `sqli_integration_test.go` | SQLi, CHECK de sobregiro y bloqueo optimista contra Postgres real. |

## 4. Resultados de la ejecución

### 4.1 Resumen global

- **20 tests de nivel superior, ~54 sub-tests** en `internal/security`.
- **19/20 tests PASS.** El único FAIL es **intencional** (tripwire, ver §4.2).
- Tiempo total de la corrida completa con base de datos: **~11–14 s**
  (la mayor parte es `TestSQLiPayloadsTreatedAsDataThroughHandlers`, ~7,8 s,
  por el coste del hash bcrypt en `Register`).
- **Cobertura del módulo completo aportada por la suite: 53,0 %** de
  sentencias (`-coverpkg="casino-backend/..."`), incluyendo config, database,
  handler, middleware, repository/postgres, service y pkg.

Tiempos por test (corrida de referencia, contenedor descartable en :5433):

| Test | Tiempo |
|---|---|
| TestSQLiPayloadsTreatedAsDataThroughHandlers | 7,79 s |
| TestOversizedUsernameAcceptedByHandler | 0,85 s |
| TestOversizedBodyNoMaxBytesReader | 0,71 s |
| TestSQLiPayloadsTreatedAsDataAgainstRealPostgres | 0,13 s |
| TestWalletOptimisticLockVersionConflict | 0,08 s |
| TestWalletOverdrawBlockedByCheckConstraint | 0,07 s |
| TestJWTForgeryRejected | 0,04 s |
| TestBackdoorScan | 0,10 s |
| Resto (12 tests) | < 0,01 s c/u |

### 4.2 El único FAIL de la suite (esperado)

```
--- FAIL: TestBackdoorScan/default_JWT_secret
    backdoor_scan_test.go:115: pattern "default JWT secret" found in production source:
    [internal\config\config.go:77]
```

Es un **tripwire deliberado**: falla mientras exista el secreto JWT por defecto
en producción (`config.go:77`, también `docker-compose.yml:33`). Es la
evidencia permanente del hallazgo **#1 (CRÍTICA)**. Cuando el equipo lo corrija
(cargar el secreto desde una variable de entorno obligatoria), el sub-test
pasará y la suite quedará 20/20 en verde.

## 5. Resultados por área de análisis (Bloque B)

| Herramienta | Resultado |
|---|---|
| `go vet ./...` | **Limpio (exit 0)** en todo el módulo. |
| `go test -race ./...` | **No ejecutable localmente** (CGO deshabilitado, sin `gcc`). Ejecutar en CI. |
| `govulncheck ./...` | **3 vulnerabilidades alcanzables** (ver §6). |
| Escaneo de secretos (password=, secret=, claves privadas, `sk_live_`, `AKIA*`) | **Sin secretos reales.** Solo constantes de test (`"test-secret"`, `"correct-horse-battery-staple-unit-test"`). |
| Escaneo de backdoors (exec, dial, HTTP client, rutas de admin/debug/pprof) | **Limpio.** Ningún backdoor ni ruta de administración oculta. |

## 6. Vulnerabilidades de dependencias (govulncheck)

Las 3 afectan al código alcanzado; se corrigen **actualizando dependencias**:

| ID | Módulo | Version actual | Versión fija | Impacto |
|---|---|---|---|---|
| GO-2026-5970 | `golang.org/x/text` | v0.16.0 | v0.39.0 | Bucle infinito con entrada inválida (alcanzado vía `pgxpool.NewWithConfig`). |
| GO-2026-5004 | `github.com/jackc/pgx/v5` | v5.6.0 | v5.9.2 | Inyección SQL por confusión de placeholders con literales de cadena entre comillas `$`. |
| GO-2025-3553 | `github.com/golang-jwt/jwt/v5` | v5.2.1 | v5.2.2 | Asignación excesiva de memoria durante el parseo de cabeceras. |

Además hay 4 vulnerabilidades en paquetes importados y 19 en módulos requeridos
que el código no llega a invocar (no son accionables en este momento).

> **Nota sobre GO-2026-5004:** la suite de SQLi confirma que todas las consultas
> del repositorio usan parámetros `$n` ligados (el payload se trata como dato).
> La actualización a `pgx >= v5.9.2` es recomendada igualmente como defensa en
> profundidad para la capa de sanitización del propio driver.

## 7. Veredicto por hallazgo de la auditoría

Leyenda: ✅ **MITIGADO** · 🔴 **CONFIRMADO** · 🟡 **PARCIAL** · ⚪ **NO VERIFICADO**

| # | Severidad | Hallazgo | Veredicto | Evidencia |
|---|---|---|---|---|
| 1 | CRÍTICA | Secreto JWT por defecto en producción | 🔴 CONFIRMADO | `config.go:77`, `docker-compose.yml:33`; tripwire `TestBackdoorScan/default_JWT_secret` falla a propósito. |
| 2 | CRÍTICA | Refresh tokens stateless, sin revocación/rotación | 🔴 CONFIRMADO | `service/auth.go:114-147`; tabla `refresh_tokens` (`migrations/003_auth.sql`) sin uso; tripwire `finding 2`. |
| 3 | ALTA | Control de acceso por claims sin verificar `is_active` | 🟡 PARCIAL | El middleware Auth rechaza tokens inválidos (7 casos PASS) y el claim `sub` aísla el IDOR, pero `IsActive` jamás se lee (`service/auth.go:73`); tripwire `finding 4`. |
| 4 | ALTA | `is_active` nunca comprobado en login/refresh | 🔴 CONFIRMADO | `service/auth.go:90-106,114-147,154-182`; tripwire `finding 4`. |
| 5 | ALTA | RNG del juego usa `math/rand` | 🔴 CONFIRMADO | `service/game.go:6,82`; tripwire `finding 5`. |
| 6 | ALTA | Operaciones de wallet no atómicas | 🔴 CONFIRMADO | `service/wallet.go:44+58,80+94`; sin transacción; tripwire `finding 6`. |
| 7 | MEDIA | Dinero como `float64` | 🔴 CONFIRMADO | `domain/wallet.go:13,37`; tripwire `finding 7`. |
| 8 | MEDIA | Sin límite de tamaño de body (`MaxBytesReader`) | 🔴 CONFIRMADO | `TestOversizedBodyNoMaxBytesReader` (body de 5 MB aceptado). |
| 9 | MEDIA | Tags `validate` decorativos | 🔴 CONFIRMADO | `TestOversizedUsernameAcceptedByHandler` (username de 1 MB aceptado). |
| 10 | MEDIA | Fuerza bruta de registro (409 en username/email existente) | ⚪ NO VERIFICADO | No testado en esta suite; requiere decisión de diseño (¿unificar 400/409?). |
| 11 | MEDIA | Coste de bcrypt doble en Register | ⚪ NO VERIFICADO | Se midió el coste del hash; la duplicación de hashing no se testea (diseño). |
| 12 | MEDIA | JWT sin `iss`/`aud`/`jti` | 🔴 CONFIRMADO | `TestJWTLacksIssAudJti`. |
| 13 | MEDIA | Refresh sin usar la tabla `refresh_tokens` | 🔴 CONFIRMADO | Subsumido por #2. |
| 14 | MEDIA | `err.Error()` crudo filtrado al cliente | 🔴 CONFIRMADO | `game_handler.go:114`, `wallet_handler.go:64,94`; tripwire `finding 14`. |
| 15 | BAJA | Rate limit sin configuración por entorno | 🟡 PARCIAL | El middleware funciona (burst 429 PASS) y rechaza ráfagas; ver #16. |
| 16 | BAJA | `X-Forwarded-For` espoofeable para saltar rate limit | 🔴 CONFIRMADO | `ratelimit.go:48-51`; `TestRateLimitXForwardedForSpoofBypass`. |
| 17 | BAJA | `Cleanup()` del rate limiter nunca invocado (fuga de memoria) | 🔴 CONFIRMADO | `ratelimit.go:98-111`; 0 % de cobertura en `Cleanup`. |
| 18 | BAJA | Error de wallet ignorado en Register | ⚪ NO VERIFICADO | No testado en esta suite. |
| 19 | BAJA | Sin logging sensible / logging con Logger noop | 🟡 PARCIAL | `logging.go` con `Logger` noop por defecto; no hay fuga observada. |
| 20 | BAJA | CORS custom sin usar | ⚪ NO VERIFICADO | `middleware/cors.go` sin cobertura (0 %); `TestSecurityHeadersThroughFullRouter` valida cabeceras. |

### Defensas confirmadas por la suite (lo que SÍ está bien)

- **Falsificación JWT rechazada**: secreto incorrecto, `sub` alterado sin
  re-firma, `alg=none`, RSA firmado con secreto HMAC, tokens expirados,
  uso cruzado access/refresh, basura y vacío → todo rechazado
  (`TestJWTForgeryRejected`, 9 vectores).
- **Sin IDOR por claims**: el `sub` del token aísla el recurso; wallet ajena
  → 403 (`TestWalletIDORPreventedByClaimsBoundary`).
- **SQLi como datos**: payloads OWASP clásicos, `DROP TABLE` y `pg_sleep`
  apilado se tratan literalmente; esquema e integridad intactos tanto por
  handlers como contra Postgres real (parámetros `$n` en todos los repos).
- **Integridad de wallet en base de datos**: `CHECK (balance >= 0)`
  (`001_init.sql:30`) bloquea sobregiros; el bloqueo optimista de versión
  evita la pérdida de dinero en escrituras concurrentes.
- **Cabeceras de seguridad** presentes y servidas por el router real.
- **Sin backdoors ni secretos reales** en el repositorio.

## 8. Recomendaciones priorizadas

1. **[CRÍTICA] Eliminar el secreto JWT por defecto** (`config.go:77`,
   `docker-compose.yml:33`): exigir `JWT_SECRET` desde entorno y fallar el
   arranque si está ausente o es el valor por defecto. El tripwire
   `TestBackdoorScan/default_JWT_secret` validará el fix automáticamente.
2. **[CRÍTICA] Actualizar dependencias**: `pgx/v5` → ≥ v5.9.2,
   `golang-jwt/jwt/v5` → ≥ v5.2.2, `golang.org/x/text` → ≥ v0.39.0.
3. **[ALTA] Revocar/rotar refresh tokens** usando la tabla `refresh_tokens`
   existente (family de sesión, `reuse` detection).
4. **[ALTA] Comprobar `is_active`** en Login/Refresh/ValidateToken.
5. **[ALTA] Wallet atómica**: envolver `UpdateBalance` + `CreateTransaction`
   en una única transacción; usar enteros centavo y/o `numeric` en lugar de
   `float64`.
6. **[ALTA] RNG criptográfico** (`crypto/rand`) para resolver apuestas.
7. **[MEDIA] `http.MaxBytesReader`** en los handlers y validación real de
   tamaños/longitudes (no solo tags).
8. **[MEDIA] No filtrar `err.Error()`** al cliente; mapear a errores genéricos.
9. **[BAJA] Rate limit** por IP real (tras proxy de confianza), invocar
   `Cleanup()` periódicamente y no confiar en `X-Forwarded-For`.
10. **[BAJA] Añadir `-race`, `govulncheck` y este paquete de tests al CI.**
