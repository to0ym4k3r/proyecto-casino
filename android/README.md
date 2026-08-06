# Casino Android App

App Android nativa para la plataforma Casino Online. Desarrollada con Kotlin y Jetpack Compose siguiendo Material Design 3.

## Stack

- **Kotlin** con **Jetpack Compose** (UI declarativa)
- **Material Design 3** (tokens de DESIGN.md)
- **Hilt** (inyeccion de dependencias)
- **Navigation Compose** (navegacion entre pantallas)
- **MVVM** (Model-View-ViewModel)
- **Retrofit** (comunicacion HTTP con el backend)

## Prerequisitos

- **Android Studio** Hedgehog (2023.1.1) o superior
- **JDK** 17
- **Android SDK** 35 (compileSdk 35)
- **API Minima**: Android 8.0 (API 26)
- **Dispositivo/Emulador**: Android 8.0+

## Setup

```bash
# 1. Abrir en Android Studio
# File -> Open -> seleccionar la carpeta android/

# 2. Sincronizar Gradle
# Android Studio sincronizara automaticamente

# 3. Configurar URL del backend
# Editar android/app/src/main/java/com/casino/app/di/AppModule.kt
# Cambiar BASE_URL a la direccion de tu backend

# 4. Ejecutar
# Shift+F10 o Run -> Run 'app'
```

## Arquitectura

```
android/app/src/main/java/com/casino/app/
+-- CasinoApp.kt              <- Application class (Hilt)
+-- MainActivity.kt           <- Activity principal (setContent)
+-- di/
|   +-- AppModule.kt          <- Modulo Hilt (Retrofit, repositorios)
+-- ui/
|   +-- navigation/
|   |   +-- CasinoNavGraph.kt <- Grafo de navegacion (NavHost)
|   +-- screens/
|   |   +-- lobby/
|   |   |   +-- LobbyScreen.kt    <- Grid de juegos (pantalla principal)
|   |   |   +-- LobbyViewModel.kt <- ViewModel del lobby
|   |   +-- game/
|   |   |   +-- GameScreen.kt     <- Pantalla de juego
|   |   +-- wallet/
|   |   |   +-- WalletScreen.kt   <- Balance y transacciones
|   |   +-- profile/
|   |       +-- ProfileScreen.kt  <- Perfil de usuario
|   +-- theme/
|       +-- Color.kt        <- Colores del design system
|       +-- Type.kt         <- Tipografia
|       +-- Shape.kt        <- Formas/bordes redondeados
|       +-- Theme.kt        <- Tema Material3 oscuro
```

### Patron de diseno
- **MVVM**: Screen -> ViewModel -> Repository -> API
- **DI**: Hilt inyecta dependencias via `@HiltViewModel`, `@Module`, `@Provides`
- **Navegacion**: Navegacion declarativa con `NavHost` y rutas tipadas

## Pantallas

| Pantalla | Descripcion |
|----------|------------|
| **Lobby** | Grid de juegos disponibles con busqueda y filtros por categoria |
| **Game** | Pantalla de juego activo con controles y saldo |
| **Wallet** | Balance actual, historial de transacciones, depositar/retirar |
| **Profile** | Informacion de usuario, configuracion, limites de juego |

## Design System

Los colores, tipografia y componentes siguen el DESIGN.md del proyecto:

- **Colores primarios**: Verde (#1DB954), Dorado (#F0B90B)
- **Fondo oscuro**: #0f1923 (bg-deep), #1a2c38 (bg-primary)
- **Texto principal**: White + Grey-100
- **Bordes redondeados**: 6-16dp segun componente

Ver [DESIGN.md](../DESIGN.md) para el design system completo.

## Build

```bash
# Debug
./gradlew assembleDebug

# Release (requiere keystore)
./gradlew assembleRelease

# Tests unitarios
./gradlew test

# Tests instrumentados
./gradlew connectedAndroidTest

# Lint
./gradlew lint
```

## Dependencias Principales

| Dependencia | Proposito |
|-------------|-----------|
| Compose BOM | UI declarativa |
| Material3 | Design system |
| Navigation Compose | Navegacion |
| Hilt | DI |
| Retrofit | HTTP client |
| OkHttp | HTTP interceptor |
| Coil | Carga de imagenes |
| Lifecycle | ViewModel + State |
