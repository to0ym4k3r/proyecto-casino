# Politica de Seguridad - Casino Online

## Compromiso

La seguridad de nuestros usuarios y la integridad de los juegos son prioridades absolutas. Este documento describe las medidas implementadas y las politicas de seguridad del proyecto.

---

## Medidas de Seguridad Implementadas

### 1. Proteccion contra SQL Injection
- **Todas** las queries usan parametros posicionales ($1, $2...)
- Cero concatenacion de strings en queries SQL
- Validacion de tipos en el driver pgx

### 2. Hashing de Contrasenas
- bcrypt con cost factor 12 (12 rondas de hashing)
- Nunca se almacenan contrasenas en texto plano
- Comparacion segura con `bcrypt.CompareHashAndPassword`

### 3. Autenticacion JWT
- **Access Token**: 15 minutos de duracion (minimo necesario)
- **Refresh Token**: 7 dias con rotacion
- Firma HMAC-SHA256 con secreto configurable
- Validacion de expiracion en cada request

### 4. Rate Limiting
- Token bucket por direccion IP
- **Auth endpoints**: 5 requests/minuto (proteccion contra fuerza bruta)
- **Endpoints generales**: 100 requests/minuto
- Limpieza automatica de buckets inactivos

### 5. Headers de Seguridad
- `Content-Security-Policy`: Politicas estrictas de contenido
- `Strict-Transport-Security`: Forzar HTTPS (HSTS) con 2 anos de duracion
- `X-Frame-Options`: DENY (prevenir clickjacking)
- `X-Content-Type-Options`: nosniff
- `X-XSS-Protection`: 1; mode=block
- `Referrer-Policy`: strict-origin-when-cross-origin
- `Permissions-Policy`: Sin acceso a camara, microfono, geolocalizacion ni pagos

### 6. CORS
- Origenes permitidos configurables
- Por defecto: solo localhost durante desarrollo
- Metodos y headers explicitos

### 7. Variables Sensibles
- Secrets en variables de entorno, nunca en codigo
- `.env` excluido de Git (`.gitignore`)
- Docker secrets para produccion

### 8. Crypto Wallet Security
- **Hot Wallet Key Management**:
  - MVP: `SOLANA_HOT_WALLET_KEY` env var (Base58 encoded private key)
  - Production: Upgrade path to HSM (AWS KMS, GCP Cloud HSM, HashiCorp Vault Transit)
  - Key NEVER in code, NEVER in git, NEVER in logs
  - `.env` excluded via `.gitignore`
- **HD Wallet Derivation**:
  - BIP44 path: `m/44'/501'/{userId}'/0'`
  - Deterministic address generation from master mnemonic
  - No per-user private keys stored — re-derive on demand for sweep
  - User index allocated atomically via DB sequence
- **Deposit Detection**:
  - Polling every 10s via Solana RPC
  - SOL: `GetSignaturesForAddress` + `GetTransaction`
  - USDC: `GetTokenAccountsByOwner` + `GetTokenAccountBalance`
  - Wait for Finalized commitment (~30s) before crediting
- **Withdrawal Security**:
  - 3-tier rate limiting by KYC level (unverified: $500/tx, L1: $5K, L2: $25K)
  - Daily withdrawal limits per tier
  - Hot wallet outflow cap: max 30% per 24h rolling window
  - Auto-approve below threshold, manual approval above
- **Emergency Pause**:
  - DB flag `emergency_paused` checked on every withdrawal
  - Activate: POST /admin/emergency/pause (requires 2-of-3 admin MFA)
  - Deactivate: 1 admin + ops lead
  - 3-level cascade for production: app kill switch → key rotation → on-chain program pause
- **Sweep Strategy**:
  - PDA-based (Solana-native) — no per-user private keys
  - Atomic batch sweeps (20-50 accounts per tx)
  - Close empty ATAs after sweep to recover rent (~0.002 SOL each)
- **Rent & Fees**:
  - System account: ~0.00089 SOL
  - Token account: ~0.00204 SOL
  - Base tx fee: 0.000005 SOL per signature
  - Priority fees: 10,000-50,000 micro-lamports/CU for fast confirmation

---

## Gambling Responsable

### Verificacion de Edad
- Registro requiere confirmacion de mayoria de edad (18+/21+ segun jurisdiccion)
- Verificacion de identidad (KYC) antes de retiros

### Limites de Deposito
- Limites diarios, semanales y mensuales configurables por usuario
- Cambios en limites tienen periodo de enfriamiento (24h para bajar, 7 dias para subir)

### Temporizador de Sesion
- Notificaciones cada 60 minutos de juego continuo
- Sesion maxima configurable
- Registro de tiempo de juego

### Autoexclusion
- Opcion de autoexclusion temporal (24h, 7 dias, 30 dias, permanente)
- Durante la autoexclusion: sin acceso a cuenta, sin depositos
- Reactivacion requiere verificacion adicional

### Historial de Actividad
- Registro completo de: depositos, retiros, apuestas, ganancias/perdidas
- Disponible para el usuario en cualquier momento
- Exportable en formato estandar

### Organizaciones de Ayuda
Si tu o alguien que conoces tiene problemas con el juego:
- **Internacional**: [Gamblers Anonymous](https://www.gamblersanonymous.org/)
- **Espana**: [FEJAR](https://www.fundacionfejar.org/)
- **Linea de ayuda**: Disponible 24/7

---

## KYC/AML (Conozca a Su Cliente / Antilavado de Dinero)

- Verificacion de identidad antes de retiros significativos
- Monitoreo de patrones de transacciones inusuales
- Retencion de registros segun regulaciones locales
- Reportes de transacciones sospechosas a autoridades competentes

---

## Checklist de Auditoria de Seguridad

Para cada release, verificar:

- [ ] Todas las queries SQL usan parametros posicionales
- [ ] No hay secrets hardcoded en el codigo fuente
- [ ] `.env` esta en `.gitignore`
- [ ] bcrypt cost factor >= 12
- [ ] JWT access token expira en <= 15 minutos
- [ ] Rate limiting activo en todos los endpoints
- [ ] Headers de seguridad configurados
- [ ] CORS restringido a origenes conocidos
- [ ] Logs no contienen informacion sensible (passwords, tokens)
- [ ] Dependencias actualizadas (sin CVEs conocidos)
- [ ] Tests de seguridad ejecutados
- [ ] Gambling responsable: limites y autoexclusion funcionales
- [ ] Solana hot wallet key stored in env var (not hardcoded)
- [ ] HD wallet derivation uses correct BIP44 path
- [ ] Withdrawal rate limits enforced per KYC tier
- [ ] Hot wallet outflow cap (30%/24h) is active
- [ ] Emergency pause mechanism tested
- [ ] Empty ATAs closed after sweep (rent recovery)

---

## Reporte de Vulnerabilidades

Si descubres una vulnerabilidad de seguridad:

1. **NO** la publiques en issues publicas
2. Envia un email a: [security@casino-online.com] (placeholder)
3. Incluye: descripcion, pasos para reproducir, impacto estimado
4. Respuesta garantizada en 48 horas
5. Credito en los release notes (si lo deseas)

---

## Ultima Revision

- Fecha: 2026-08-03
- Revisado por: Equipo de Desarrollo Casino Online
