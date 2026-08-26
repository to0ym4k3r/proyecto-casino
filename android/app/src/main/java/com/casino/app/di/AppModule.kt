package com.casino.app.di

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.preferencesDataStore
import com.casino.app.BuildConfig
import com.casino.app.data.api.AuthApi
import com.casino.app.data.api.AuthInterceptor
import com.casino.app.data.api.GameApi
import com.casino.app.data.api.KycApi
import com.casino.app.data.api.UserApi
import com.casino.app.data.api.BonusApi
import com.casino.app.data.api.WalletApi
import com.casino.app.data.auth.AuthRepository
import com.casino.app.data.bonus.BonusRepository
import com.casino.app.data.game.GameRepository
import com.casino.app.data.kyc.KycRepository
import com.casino.app.data.profile.ProfileRepository
import com.casino.app.data.token.TokenStore
import com.casino.app.data.wallet.WalletRepository
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.android.qualifiers.ApplicationContext
import dagger.hilt.components.SingletonComponent
import java.util.concurrent.TimeUnit
import javax.inject.Singleton
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import retrofit2.Retrofit
import retrofit2.converter.kotlinx.serialization.asConverterFactory

/**
 * Singleton scope for the Preferences DataStore that persists auth tokens.
 *
 * The delegate must live at file scope (DataStore requirement); it is internal
 * so the data layer can read it when a context is available, while Hilt
 * receives the instance through [AppModule.provideDataStore].
 */
internal val Context.authDataStore: DataStore<Preferences> by preferencesDataStore(name = "auth_preferences")

/**
 * Root Hilt dependency injection module for the Casino application.
 *
 * Provides the network stack (design D8 — Retrofit + OkHttp + kotlinx
 * serialization against the frozen backend contract at `BuildConfig.API_BASE_URL`),
 * the token DataStore (design D9), the auth interceptor (design D10), and the
 * repositories as their layers land.
 *
 * @author Casino Team
 * @since 2026-08-03
 */
@Module
@InstallIn(SingletonComponent::class)
object AppModule {

    /**
     * App-wide JSON configuration. ignoreUnknownKeys keeps the client tolerant
     * of additive backend fields; explicitNulls/encodeDefaults mirror the
     * backend's `omitempty` semantics on the wire.
     */
    @Provides
    @Singleton
    fun provideJson(): Json = Json {
        ignoreUnknownKeys = true
        explicitNulls = false
        encodeDefaults = false
    }

    @Provides
    @Singleton
    fun provideOkHttpClient(authInterceptor: AuthInterceptor): OkHttpClient =
        OkHttpClient.Builder()
            .connectTimeout(15, TimeUnit.SECONDS)
            .readTimeout(30, TimeUnit.SECONDS)
            .addInterceptor(authInterceptor)
            .build()

    @Provides
    @Singleton
    fun provideRetrofit(client: OkHttpClient, json: Json): Retrofit =
        Retrofit.Builder()
            // Retrofit requires a trailing slash on the base URL.
            .baseUrl(BuildConfig.API_BASE_URL.trimEnd('/') + "/")
            .client(client)
            .addConverterFactory(json.asConverterFactory("application/json".toMediaType()))
            .build()

    @Provides
    @Singleton
    fun provideAuthApi(retrofit: Retrofit): AuthApi = retrofit.create(AuthApi::class.java)

    @Provides
    @Singleton
    fun provideUserApi(retrofit: Retrofit): UserApi = retrofit.create(UserApi::class.java)

    @Provides
    @Singleton
    fun provideKycApi(retrofit: Retrofit): KycApi = retrofit.create(KycApi::class.java)

    @Provides
    @Singleton
    fun provideDataStore(@ApplicationContext context: Context): DataStore<Preferences> = context.authDataStore

    @Provides
    @Singleton
    fun provideTokenStore(dataStore: DataStore<Preferences>): TokenStore = TokenStore(dataStore)

    @Provides
    @Singleton
    fun provideAuthInterceptor(tokenStore: TokenStore): AuthInterceptor = AuthInterceptor(tokenStore)

    /** App-wide scope that keeps [AuthRepository.authed] alive beyond any UI lifetime. */
    @Provides
    @Singleton
    fun provideAuthScope(): CoroutineScope = CoroutineScope(SupervisorJob() + Dispatchers.Default)

    @Provides
    @Singleton
    fun provideAuthRepository(
        tokenStore: TokenStore,
        authApi: AuthApi,
        scope: CoroutineScope,
    ): AuthRepository = AuthRepository(tokenStore, authApi, scope)

    @Provides
    @Singleton
    fun provideProfileRepository(userApi: UserApi): ProfileRepository =
        ProfileRepository(userApi)

    @Provides
    @Singleton
    fun provideKycRepository(kycApi: KycApi): KycRepository =
        KycRepository(kycApi)

    @Provides
    @Singleton
    fun provideWalletApi(retrofit: Retrofit): WalletApi = retrofit.create(WalletApi::class.java)

    @Provides
    @Singleton
    fun provideWalletRepository(walletApi: WalletApi): WalletRepository =
        WalletRepository(walletApi)

    @Provides
    @Singleton
    fun provideGameApi(retrofit: Retrofit): GameApi = retrofit.create(GameApi::class.java)

    @Provides
    @Singleton
    fun provideGameRepository(gameApi: GameApi): GameRepository =
        GameRepository(gameApi)

    @Provides
    @Singleton
    fun provideBonusApi(retrofit: Retrofit): BonusApi = retrofit.create(BonusApi::class.java)

    @Provides
    @Singleton
    fun provideBonusRepository(bonusApi: BonusApi): BonusRepository =
        BonusRepository(bonusApi)
}
