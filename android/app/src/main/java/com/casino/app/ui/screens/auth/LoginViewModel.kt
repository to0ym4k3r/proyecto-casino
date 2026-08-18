package com.casino.app.ui.screens.auth

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.casino.app.data.auth.AuthRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** UI state of the login screen (design: login exposes loading/error/success). */
data class LoginUiState(
    val email: String = "",
    val password: String = "",
    val isLoading: Boolean = false,
    val error: String? = null,
)

/**
 * Drives the login screen (task 5.6).
 *
 * On success it does NOT navigate: the auth gate observes the session flow and
 * switches to the main tabs itself. Wrong credentials surface the backend's
 * message as an inline error with no navigation (spec scenario). Loading state
 * disables the submit button AND guards double submission in the ViewModel.
 */
@HiltViewModel
class LoginViewModel @Inject constructor(
    private val authRepository: AuthRepository,
    private val savedStateHandle: SavedStateHandle,
) : ViewModel() {

    private val _uiState = MutableStateFlow(LoginUiState())
    val uiState: StateFlow<LoginUiState> = _uiState.asStateFlow()

    /**
     * True while the "email verified" banner should show. The verify screen
     * sets the `email_verified` flag on the login back-stack entry after a
     * successful verification (task 5.8), so the message survives the pop.
     */
    val verifiedBanner: StateFlow<Boolean> = savedStateHandle.getStateFlow(KEY_VERIFIED, false)

    fun onEmailChange(value: String) {
        _uiState.update { it.copy(email = value) }
    }

    fun onPasswordChange(value: String) {
        _uiState.update { it.copy(password = value) }
    }

    fun login() {
        val state = _uiState.value
        if (state.isLoading) return
        val email = state.email.trim()
        if (email.isBlank() || state.password.isEmpty()) {
            _uiState.update { it.copy(error = "Enter your email and password.") }
            return
        }

        _uiState.update { it.copy(isLoading = true, error = null) }
        viewModelScope.launch {
            try {
                authRepository.login(email, state.password)
                // Success: nothing to do — the gate reacts to authed == true.
                _uiState.update { it.copy(isLoading = false) }
            } catch (t: Throwable) {
                if (t is CancellationException) throw t
                _uiState.update {
                    it.copy(isLoading = false, error = AuthErrorMessages.from(t, "Invalid email or password."))
                }
            }
        }
    }

    /** Dismisses the "email verified" banner. */
    fun consumeVerifiedBanner() {
        savedStateHandle[KEY_VERIFIED] = false
    }

    private companion object {
        const val KEY_VERIFIED = "email_verified"
    }
}
