package com.casino.app.ui.screens.game

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.casino.app.data.game.GameRepository
import com.casino.app.data.api.dto.BetResultResponse
import com.casino.app.data.api.dto.GameDto
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * UI state for the game screen.
 */
data class GameUiState(
    val game: GameDto? = null,
    val isLoading: Boolean = true,
    val error: String? = null,
    val sessionId: String? = null,
    val serverSeedHash: String? = null,
    val balance: Long = 0L,
    val lastBet: BetResultResponse? = null,
    val isPlacingBet: Boolean = false,
    val betError: String? = null,
)

/**
 * ViewModel for the game screen.
 *
 * Manages game session lifecycle: loading the game, starting a session,
 * placing bets, and ending sessions.
 */
@HiltViewModel
class GameViewModel @Inject constructor(
    private val gameRepository: GameRepository,
    savedStateHandle: SavedStateHandle,
) : ViewModel() {

    private val gameId: String = savedStateHandle["gameId"]
        ?: throw IllegalArgumentException("gameId is required")

    private val _uiState = MutableStateFlow(GameUiState())
    val uiState: StateFlow<GameUiState> = _uiState.asStateFlow()

    init {
        loadGame()
        startSession()
    }

    private fun loadGame() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }
            try {
                val game = gameRepository.getGame(gameId)
                _uiState.update { it.copy(game = game, isLoading = false) }
            } catch (e: Exception) {
                _uiState.update {
                    it.copy(isLoading = false, error = e.localizedMessage ?: "Failed to load game")
                }
            }
        }
    }

    private fun startSession() {
        viewModelScope.launch {
            try {
                val session = gameRepository.startGame(gameId)
                _uiState.update {
                    it.copy(
                        sessionId = session.sessionId,
                        serverSeedHash = session.serverSeedHash,
                    )
                }
            } catch (e: Exception) {
                _uiState.update {
                    it.copy(error = e.localizedMessage ?: "Failed to start session")
                }
            }
        }
    }

    /**
     * Places a bet with the given amount (in integer cents).
     */
    fun placeBet(amount: Long) {
        if (_uiState.value.isPlacingBet) return

        viewModelScope.launch {
            _uiState.update { it.copy(isPlacingBet = true, betError = null, lastBet = null) }
            try {
                val result = gameRepository.placeBet(gameId, amount)
                _uiState.update {
                    it.copy(
                        lastBet = result,
                        isPlacingBet = false,
                    )
                }
            } catch (e: Exception) {
                _uiState.update {
                    it.copy(
                        isPlacingBet = false,
                        betError = e.localizedMessage ?: "Failed to place bet",
                    )
                }
            }
        }
    }

    /**
     * Ends the current game session.
     */
    fun endSession() {
        viewModelScope.launch {
            try {
                gameRepository.endSession(gameId)
                _uiState.update {
                    it.copy(sessionId = null, serverSeedHash = null, lastBet = null)
                }
            } catch (e: Exception) {
                _uiState.update {
                    it.copy(error = e.localizedMessage ?: "Failed to end session")
                }
            }
        }
    }

    /**
     * Clears the last bet result (e.g., after the user acknowledges it).
     */
    fun clearBetResult() {
        _uiState.update { it.copy(lastBet = null, betError = null) }
    }
}
