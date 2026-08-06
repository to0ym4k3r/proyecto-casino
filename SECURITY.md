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
