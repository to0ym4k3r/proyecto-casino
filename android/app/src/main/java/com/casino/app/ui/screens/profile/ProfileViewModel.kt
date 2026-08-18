package com.casino.app.ui.screens.profile

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.casino.app.data.profile.ProfileRepository
import com.casino.app.data.api.dto.UpdateProfileRequest
import com.casino.app.ui.screens.auth.AuthErrorMessages
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** UI state of the profile screen (loading / error / editable fields). */
data class ProfileUiState(
    val isLoading: Boolean = true,
    val error: String? = null,
    val isSaving: Boolean = false,
    val id: String = "",
    val email: String = "",
    val username: String = "",
    val firstName: String = "",
    val lastName: String = "",
)

/**
 * ViewModel for the Profile screen (task 6.2).
 *
 * Loads the profile on construction via GET /users/me and exposes editable
 * fields. On save, it calls PUT /users/me with only the changed fields
 * (design D6 — email is immutable).
 *
 * Navigation: none. The profile tab is already selected; saving simply
 * refreshes the in-place data.
 */
@HiltViewModel
class ProfileViewModel @Inject constructor(
    private val profileRepository: ProfileRepository,
) : ViewModel() {

    private val _uiState = MutableStateFlow(ProfileUiState())
    val uiState: StateFlow<ProfileUiState> = _uiState.asStateFlow()

    init {
        loadProfile()
    }

    fun onUsernameChange(value: String) {
        _uiState.update { it.copy(username = value) }
    }

    fun onFirstNameChange(value: String) {
        _uiState.update { it.copy(firstName = value) }
    }

    fun onLastNameChange(value: String) {
        _uiState.update { it.copy(lastName = value) }
    }

    fun save() {
        val state = _uiState.value
        if (state.isSaving) return

        _uiState.update { it.copy(isSaving = true, error = null) }
        viewModelScope.launch {
            try {
                val updated = profileRepository.updateProfile(
                    UpdateProfileRequest(
                        username = state.username.trim(),
                        firstName = state.firstName.trim(),
                        lastName = state.lastName.trim(),
                    )
                )
                _uiState.update {
                    it.copy(
                        isSaving = false,
                        username = updated.username,
                        firstName = updated.firstName,
                        lastName = updated.lastName,
                    )
                }
            } catch (t: Throwable) {
                if (t is CancellationException) throw t
                _uiState.update {
                    it.copy(isSaving = false, error = AuthErrorMessages.from(t, "Failed to save profile."))
                }
            }
        }
    }

    private fun loadProfile() {
        viewModelScope.launch {
            try {
                val user = profileRepository.getProfile()
                _uiState.update {
                    it.copy(
                        isLoading = false,
                        id = user.id,
                        email = user.email,
                        username = user.username,
                        firstName = user.firstName,
                        lastName = user.lastName,
                    )
                }
            } catch (t: Throwable) {
                if (t is CancellationException) throw t
                _uiState.update {
                    it.copy(
                        isLoading = false,
                        error = AuthErrorMessages.from(t, "Failed to load profile."),
                    )
                }
            }
        }
    }
}
