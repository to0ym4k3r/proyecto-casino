package com.casino.app.data.token

import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import com.casino.app.data.api.dto.TokenPair
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map

/**
 * Persists the auth token pair in Preferences DataStore (design D9).
 *
 * The stored tokens are the single source of truth for session state: the
 * auth gate derives its start destination from token presence, and clearing
 * the store forces login. Backing this with DataStore (not SharedPreferences)
 * satisfies the spec's token-storage requirement.
 *
 * [TokenStore] takes the [DataStore] as a constructor dependency so unit tests
 * can drive it with a temp-file store; Hilt supplies the app-wide instance
 * through [com.casino.app.di.AppModule].
 */
class TokenStore(private val dataStore: DataStore<Preferences>) {

    /** The persisted access token, or null when no session exists. */
    val accessToken: Flow<String?> = dataStore.data.map { it[KEY_ACCESS_TOKEN] }

    /** The persisted refresh token, or null when no session exists. */
    val refreshToken: Flow<String?> = dataStore.data.map { it[KEY_REFRESH_TOKEN] }

    /**
     * The persisted token pair, or null when either half is missing. Auth code
     * (e.g. [com.casino.app.data.auth.AuthRepository]) observes this to keep
     * the authenticated state in sync, including interceptor-initiated clears.
     */
    val tokens: Flow<TokenPair?> = dataStore.data.map { prefs ->
        val access = prefs[KEY_ACCESS_TOKEN]
        val refresh = prefs[KEY_REFRESH_TOKEN]
        if (access != null && refresh != null) TokenPair(access, refresh) else null
    }

    /** Persists the token pair. A subsequent app start restores the session. */
    suspend fun saveTokens(pair: TokenPair) {
        dataStore.edit { prefs ->
            prefs[KEY_ACCESS_TOKEN] = pair.accessToken
            prefs[KEY_REFRESH_TOKEN] = pair.refreshToken
        }
    }

    /** Removes both tokens; the auth gate immediately shows login. */
    suspend fun clear() {
        dataStore.edit { prefs ->
            prefs.remove(KEY_ACCESS_TOKEN)
            prefs.remove(KEY_REFRESH_TOKEN)
        }
    }

    private companion object {
        val KEY_ACCESS_TOKEN = stringPreferencesKey("access_token")
        val KEY_REFRESH_TOKEN = stringPreferencesKey("refresh_token")
    }
}
