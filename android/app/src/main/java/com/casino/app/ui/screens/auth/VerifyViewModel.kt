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

/** UI state of the email verification screen. */
data class VerifyUiState(
    val token: String = "",
    val isLoading: Boolean = false,
    val error: String? = null,
    val verified: Boolean = false,
)

/**
 * Drives the email verification screen (task 5.8).
 *
 * The `casino://verify?token={token}` deep link arrives as the `token` route
 * argument in the [SavedStateHandle], so verification starts automatically in
 * [init]. Users opening the screen without a deep link (e.g. after register)
 * can paste the token from their email as a fallback. Success does NOT
 * navigate: the auth graph reads [verified] and pops back to login with an
 * "email verified" flag.
 */
@HiltViewModel
class VerifyViewModel @Inject constructor(
    private val authRepository: AuthRepository,
    savedStateHandle: SavedStateHandle,
) : ViewModel() {

    private val _uiState = MutableStateFlow(VerifyUiState())
    val uiState: StateFlow<VerifyUiState> = _uiState.asStateFlow()

    init {
        val deepLinkToken: String? = savedStateHandle[KEY_TOKEN]
        if (!deepLinkToken.isNullOrBlank()) {
            verify(deepLinkToken)
        }
    }

    fun onTokenChange(value: String) {
        _uiState.update { it.copy(token = value) }
    }

    /** Verifies [token] (defaults to the pasted field value). */
    fun verify(token: String = _uiState.value.token) {
        val state = _uiState.value
        if (state.isLoading || state.verified) return
        val trimmed = token.trim()
        if (trimmed.isEmpty()) {
            _uiState.update { it.copy(error = "Paste the verification token from your email.") }
            return
        }

        _uiState.update { it.copy(isLoading = true, error = null) }
        viewModelScope.launch {
            try {
                authRepository.verifyEmail(trimmed)
                _uiState.update { it.copy(isLoading = false, verified = true) }
            } catch (t: Throwable) {
                if (t is CancellationException) throw t
                _uiState.update {
                    it.copy(
                        isLoading = false,
                        error = AuthErrorMessages.from(t, "Verification failed. Check the token and try again."),
                    )
                }
            }
        }
    }

    private companion object {
        const val KEY_TOKEN = "token"
    }
}
