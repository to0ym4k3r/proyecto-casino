package com.casino.app

import android.app.Application
import dagger.hilt.android.HiltAndroidApp

/**
 * Application class for the Casino app.
 *
 * Annotated with [HiltAndroidApp] to enable Hilt dependency injection
 * across the entire application lifecycle. This class serves as the
 * entry point for Hilt's component tree and must be registered in the
 * AndroidManifest.xml.
 *
 * @author Casino Team
 * @since 2026-08-03
 */
@HiltAndroidApp
class CasinoApp : Application()
