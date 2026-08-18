package com.casino.app.ui.navigation

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.casino.app.data.auth.AuthRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.stateIn

/**
 * The graph the auth gate renders for a given session state.
 *
 * [Splash] holds the app root while the gate waits for the FIRST real emission
 * of the token store (deviation #12: `AuthRepository.authed` carries a
 * synthetic initial `false`, so a pre-existing session must not flash login).
 */
enum class AuthGateDestination {
    Splash,
    Main,
    Auth,
    ;

    companion object {
        /**
         * Pure mapping from raw session state to the gate destination:
         * `null` = not resolved yet (hold), `true` = authenticated (main tabs),
         * `false` = unauthenticated (auth flow).
         */
        fun fromSession(authenticated: Boolean?): AuthGateDestination = when (authenticated) {
            null -> Splash
            true -> Main
            false -> Auth
        }
    }
}

/**
 * Drives the navigation gate (design D9, task 5.5).
 *
 * OBSERVES [AuthRepository.session] — the raw store-derived flow whose first
 * emission is always the real session state — and exposes the derived
 * [destination]. The NavGraph switches its content from this flow, so a
 * persisted session resolves straight to the main tabs without flashing login,
 * and a logout (or interceptor-initiated 401 clear) returns to the auth flow.
 */
@HiltViewModel
class AuthGateViewModel @Inject constructor(
    repository: AuthRepository,
) : ViewModel() {

    val destination: StateFlow<AuthGateDestination> = repository.session
        .map { AuthGateDestination.fromSession(it) }
        .stateIn(viewModelScope, SharingStarted.Eagerly, AuthGateDestination.Splash)
}
