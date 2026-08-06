package com.casino.app.di

import dagger.Module
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent

/**
 * Root Hilt dependency injection module for the Casino application.
 *
 * Installed in [SingletonComponent] to provide application-scoped dependencies.
 * Additional modules can be added here or as separate objects annotated with
 * [Module] and scoped to appropriate Hilt components.
 *
 * @author Casino Team
 * @since 2026-08-03
 */
@Module
@InstallIn(SingletonComponent::class)
object AppModule
