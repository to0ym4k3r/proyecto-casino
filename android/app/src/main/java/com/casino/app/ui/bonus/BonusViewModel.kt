package com.casino.app.ui.bonus

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.casino.app.data.api.dto.ActiveBonusDto
import com.casino.app.data.api.dto.BonusDefinitionDto
import com.casino.app.data.bonus.BonusRepository
import com.casino.app.ui.util.formatCentsAsDollars
import com.casino.app.ui.screens.auth.AuthErrorMessages
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/**
 * UI state for the bonus screens.
 *
 * Covers available bonuses, active bonuses, claim flow, and forfeit flow.
 * All monetary amounts are in integer cents.
 */
data class BonusUiState(
    // Available bonuses
    val isLoadingAvailable: Boolean = false,
    val availableBonuses: List<BonusDefinitionDto> = emptyList(),
    val availableError: String? = null,

    // Active bonuses
    val isLoadingActive: Boolean = false,
    val activeBonuses: List<ActiveBonusDto> = emptyList(),
    val activeError: String? = null,

    // Claim flow
    val isClaiming: Boolean = false,
    val claimError: String? = null,
    val lastClaimedGrantId: String? = null,

    // Forfeit flow
    val isForfeiting: Boolean = false,
    val forfeitError: String? = null,
    val forfeitedGrantId: String? = null,
)

/**
 * ViewModel for the Bonus screens (tasks 6.5).
 *
 * Exposes loadAvailable, claimBonus, loadActive, and forfeitBonus actions.
 * Uses StateFlow for reactive UI updates.
 *
 * Amounts use integer cents on the wire (e.g. 50000 = $500.00).
 */
@HiltViewModel
class BonusViewModel @Inject constructor(
    private val bonusRepository: BonusRepository,
) : ViewModel() {

    private val _uiState = MutableStateFlow(BonusUiState())
    val uiState: StateFlow<BonusUiState> = _uiState.asStateFlow()

    init {
        loadAvailable()
        loadActive()
    }

    // --- Available bonuses ---

    /** Loads claimable bonus definitions from the backend. */
    fun loadAvailable() {
        _uiState.update { it.copy(isLoadingAvailable = true, availableError = null) }
        viewModelScope.launch {
            try {
                val bonuses = bonusRepository.getAvailableBonuses()
                _uiState.update {
                    it.copy(
                        isLoadingAvailable = false,
                        availableBonuses = bonuses,
                    )
                }
            } catch (t: Throwable) {
                if (t is CancellationException) throw t
                _uiState.update {
                    it.copy(
                        isLoadingAvailable = false,
                        availableError = AuthErrorMessages.from(t, "Failed to load available bonuses."),
                    )
                }
            }
        }
    }

    // --- Claim bonus ---

    /** Claims a bonus by its definition ID. */
    fun claimBonus(definitionId: String) {
        _uiState.update { it.copy(isClaiming = true, claimError = null) }
        viewModelScope.launch {
            try {
                val result = bonusRepository.claimBonus(definitionId)
                _uiState.update {
                    it.copy(
                        isClaiming = false,
                        lastClaimedGrantId = result.grantId,
                    )
                }
                // Refresh both lists after successful claim
                loadAvailable()
                loadActive()
            } catch (t: Throwable) {
                if (t is CancellationException) throw t
                _uiState.update {
                    it.copy(
                        isClaiming = false,
                        claimError = AuthErrorMessages.from(t, "Failed to claim bonus."),
                    )
                }
            }
        }
    }

    // --- Active bonuses ---

    /** Loads the user's active bonus grants with wagering progress. */
    fun loadActive() {
        _uiState.update { it.copy(isLoadingActive = true, activeError = null) }
        viewModelScope.launch {
            try {
                val bonuses = bonusRepository.getActiveBonuses()
                _uiState.update {
                    it.copy(
                        isLoadingActive = false,
                        activeBonuses = bonuses,
                    )
                }
            } catch (t: Throwable) {
                if (t is CancellationException) throw t
                _uiState.update {
                    it.copy(
                        isLoadingActive = false,
                        activeError = AuthErrorMessages.from(t, "Failed to load active bonuses."),
                    )
                }
            }
        }
    }

    // --- Forfeit bonus ---

    /** Forfeits an active bonus grant by its grant ID. */
    fun forfeitBonus(grantId: String) {
        _uiState.update { it.copy(isForfeiting = true, forfeitError = null) }
        viewModelScope.launch {
            try {
                bonusRepository.forfeitBonus(grantId)
                _uiState.update {
                    it.copy(
                        isForfeiting = false,
                        forfeitedGrantId = grantId,
                    )
                }
                // Refresh active list after successful forfeit
                loadActive()
            } catch (t: Throwable) {
                if (t is CancellationException) throw t
                _uiState.update {
                    it.copy(
                        isForfeiting = false,
                        forfeitError = AuthErrorMessages.from(t, "Failed to forfeit bonus."),
                    )
                }
            }
        }
    }
}
