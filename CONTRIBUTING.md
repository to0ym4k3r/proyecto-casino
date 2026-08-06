# Guia de Contribucion - Casino Online

Gracias por contribuir a Casino Online. Este documento describe las reglas y flujos de trabajo para contribuir al proyecto.

---

## Primeros Pasos

1. Forkear el repositorio
2. Clonar tu fork: `git clone <url>`
3. Crear una branch para tu cambio: `git checkout -b feature/nombre-del-cambio`
4. Hacer tus cambios
5. Push a tu fork: `git push origin feature/nombre-del-cambio`
6. Abrir un Pull Request contra `main`

---

## Estrategia de Ramas

```
main <- branch principal, siempre estable
+-- feature/nombre       <- Nuevas funcionalidades
+-- fix/nombre           <- Correccion de bugs
+-- docs/nombre          <- Cambios de documentacion
+-- refactor/nombre      <- Refactorizaciones
+-- security/nombre      <- Fixes de seguridad
+-- test/nombre          <- Mejoras de tests
```

**Reglas:**
- `main` siempre debe compilar y pasar tests
- Una branch por cambio (no mezclar features)
- Nombre descriptivo: `feature/pubnub-integration`, `fix/wallet-balance-race`

---

## Convencion de Commits

Usamos [Conventional Commits](https://www.conventionalcommits.org/):

```
<tipo>(<scope>): <descripcion corta>

[descripcion opcional mas detallada]

[footer opcional - issues cerrados]
```

**Tipos:**
- `feat`: Nueva funcionalidad
- `fix`: Correccion de bug
- `docs`: Documentacion
- `refactor`: Refactorizacion sin cambio de comportamiento
- `test`: Agregar o corregir tests
- `security`: Mejora de seguridad
- `chore`: Tareas de mantenimiento (deps, CI, etc.)

**Scopes:** `android`, `backend`, `infra`, `docs`

**Ejemplos:**
```
feat(backend): add PubNub real-time betting integration
fix(android): correct wallet balance refresh on resume
docs(backend): add API endpoint reference
security(backend): rotate JWT secret periodically
```

---

## Code Review

### Requisitos para PR
- [ ] Codigo compila sin errores (`make build` / `./gradlew assembleDebug`)
- [ ] Tests pasan (`make test` / `./gradlew test`)
- [ ] Linter sin warnings (`make lint` / `./gradlew lint`)
- [ ] Documentacion actualizada (KDoc/godoc + README si aplica)
- [ ] Commit messages siguen Conventional Commits
- [ ] No incluye secrets o credenciales

### Proceso
1. Abrir PR con titulo claro y descripcion
2. Esperar al menos 1 review approving
3. Resolver todos los comentarios
4. Squash merge en `main`

---

## Testing

### Backend (Go)
```bash
cd backend
make test          # Todos los tests
go test ./... -v   # Verbose
go test -cover     # Cobertura
```

### Android (Kotlin)
```bash
cd android
./gradlew test                    # Unit tests
./gradlew connectedAndroidTest    # Instrumented tests
```

### Reglas de testing
- Todo cambio de comportamiento debe tener test
- Bugs deben tener test que falle sin el fix
- Tests deben ser deterministas (sin dependencia de tiempo/red)

---

## Documentacion

### Requisitos
- **Funciones exportadas**: deben tener godoc (Go) o KDoc (Kotlin)
- **Archivos .md**: en espanol neutro/profesional
- **Codigo comments**: en ingles
- **API changes**: actualizar backend/README.md
- **Screens nuevas**: actualizar android/README.md

### Design System
- Seguir los tokens de colores en DESIGN.md
- No agregar colores nuevos sin actualizar DESIGN.md
- Componentes nuevos deben seguir el patron existente

---

## Seguridad

- **Nunca** hardcodear secrets, passwords, o tokens
- **Nunca** commitear `.env` o archivos con credenciales
- Queries SQL: **siempre** usar parametros posicionales
- Si encuentras una vulnerabilidad: ver [SECURITY.md](SECURITY.md)

---

## Estilo de Codigo

### Go
- Seguir [Effective Go](https://go.dev/doc/effective_go)
- `gofmt` / `goimports` para formateo
- Nombres en camelCase para funciones, PascalCase para tipos

### Kotlin
- Seguir [Kotlin Coding Conventions](https://kotlinlang.org/docs/coding-conventions.html)
- Compose: funciones PascalCase, parametros en camelCase
- KDoc en toda funcion publica

---

## Preguntas?

Abri un issue con el tag `question` o contacta al equipo de desarrollo.
