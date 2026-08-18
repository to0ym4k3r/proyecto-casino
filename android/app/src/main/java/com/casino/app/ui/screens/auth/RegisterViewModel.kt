package com.casino.app.ui.screens.auth

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.casino.app.data.api.dto.RegisterRequest
import com.casino.app.data.auth.AuthRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** UI state of the register screen (design: register exposes loading/error/success). */
data class RegisterUiState(
    val username: String = "",
    val email: String = "",
    val password: String = "",
    val firstName: String = "",
    val lastName: String = "",
    val dateOfBirth: String = "",
    val isLoading: Boolean = false,
    val fieldErrors: RegisterFieldErrors = RegisterFieldErrors(),
    val error: String? = null,
    val registered: Boolean = false,
)

/**
 * Drives the register screen (task 5.7).
 *
 * Spec scenario: "GIVEN invalid register data WHEN submitting register THEN
 * inline validation errors shown and no network call" — validation runs
 * synchronously BEFORE the repository is touched. Registration does NOT log
 * the user in (no tokens returned): on success the screen guides the user to
 * the email verification step.
 */
@HiltViewModel
class RegisterViewModel @Inject constructor(
    private val authRepository: AuthRepository,
    savedStateHandle: SavedStateHandle,
) : ViewModel() {

    private val _uiState = MutableStateFlow(RegisterUiState())
    val uiState: StateFlow<RegisterUiState> = _uiState.asStateFlow()

    fun onUsernameChange(value: String) = _uiState.update { it.copy(username = value) }

    fun onEmailChange(value: String) = _uiState.update { it.copy(email = value) }

    fun onPasswordChange(value: String) = _uiState.update { it.copy(password = value) }

    fun onFirstNameChange(value: String) = _uiState.update { it.copy(firstName = value) }

    fun onLastNameChange(value: String) = _uiState.update { it.copy(lastName = value) }

    fun onDateOfBirthChange(value: String) = _uiState.update { it.copy(dateOfBirth = value) }

    fun submit() {
        val state = _uiState.value
        if (state.isLoading || state.registered) return

        val errors = RegisterValidation.validate(
            username = state.username,
            email = state.email,
            password = state.password,
            firstName = state.firstName,
            lastName = state.lastName,
            dateOfBirth = state.dateOfBirth,
        )
        if (!errors.isEmpty) {
            _uiState.update { it.copy(fieldErrors = errors, error = null) }
            return
        }

        _uiState.update { it.copy(isLoading = true, error = null, fieldErrors = RegisterFieldErrors()) }
        viewModelScope.launch {
            try {
                authRepository.register(
                    RegisterRequest(
                        username = state.username.trim(),
                        email = state.email.trim(),
                        password = state.password,
                        firstName = state.firstName.trim(),
                        lastName = state.lastName.trim(),
                        dateOfBirth = state.dateOfBirth.trim(),
                    ),
                )
                _uiState.update { it.copy(isLoading = false, registered = true) }
            } catch (t: Throwable) {
                if (t is CancellationException) throw t
                _uiState.update {
                    it.copy(isLoading = false, error = AuthErrorMessages.from(t, "Registration failed. Please try again."))
                }
            }
        }
    }
}
