package com.casino.app.ui.screens.wallet

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.casino.app.data.api.dto.SolanaTxDto
import com.casino.app.data.wallet.WalletRepository
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
 * UI state for the wallet screen.
 *
 * Covers balance display, crypto deposit addresses, crypto withdrawals,
 * and paginated transaction history.
 */
data class WalletUiState(
    // Balance
    val isLoadingBalance: Boolean = true,
    val balanceCents: Long = 0L,
    val balanceFormatted: String = "$0.00",
    val currency: String = "USD",
    val balanceError: String? = null,

    // Deposit
    val isDepositing: Boolean = false,
    val depositAddress: String? = null,
    val depositQrData: String? = null,
    val depositError: String? = null,

    // Withdrawal
    val isWithdrawing: Boolean = false,
    val withdrawAmountInput: String = "",
    val withdrawAmountCents: Long = 0L,
    val withdrawAddress: String = "",
    val withdrawError: String? = null,
    val lastWithdrawalId: String? = null,
    val lastWithdrawalStatus: String? = null,

    // Transaction history
    val isLoadingHistory: Boolean = true,
    val transactions: List<SolanaTxDto> = emptyList(),
    val historyTotal: Int = 0,
    val historyPage: Int = 1,
    val activeFilter: String? = null,
    val historyError: String? = null,
)

/**
 * ViewModel for the Wallet screen (design tasks 5.4–5.5).
 *
 * Loads balance and transaction history on construction.
 * Exposes deposit, withdrawal, and history-filter actions.
 *
 * Amounts use integer cents on the wire (e.g. 15000 = $150.00).
 * The UI converts dollars ↔ cents via [onWithdrawAmountChange].
 */
@HiltViewModel
class WalletViewModel @Inject constructor(
    private val walletRepository: WalletRepository,
) : ViewModel() {

    private val _uiState = MutableStateFlow(WalletUiState())
    val uiState: StateFlow<WalletUiState> = _uiState.asStateFlow()

    init {
        loadBalance()
        loadTxHistory()
    }

    // --- Deposit ---

    /** Requests a SOL deposit address from the backend. */
    fun getSolDeposit() {
        _uiState.update { it.copy(isDepositing = true, depositError = null) }
        viewModelScope.launch {
            try {
                val result = walletRepository.depositSol()
                _uiState.update {
                    it.copy(
                        isDepositing = false,
                        depositAddress = result.address,
                        depositQrData = result.qrData,
                    )
                }
            } catch (t: Throwable) {
                if (t is CancellationException) throw t
                _uiState.update {
                    it.copy(
                        isDepositing = false,
                        depositError = AuthErrorMessages.from(t, "Failed to get deposit address."),
                    )
                }
            }
        }
    }

    // --- Withdrawal ---

    fun onWithdrawAmountChange(value: String) {
        val cents = parseDollarsToCents(value)
        _uiState.update { it.copy(withdrawAmountInput = value, withdrawAmountCents = cents) }
    }

    fun onWithdrawAddressChange(value: String) {
        _uiState.update { it.copy(withdrawAddress = value) }
    }

    /** Requests a SOL withdrawal. Validates input before calling the API. */
    fun withdrawSol() {
        val state = _uiState.value
        val amountCents = state.withdrawAmountCents
        val address = state.withdrawAddress.trim()

        // Validation
        if (address.isEmpty()) {
            _uiState.update { it.copy(withdrawError = "Recipient address is required.") }
            return
        }
        if (amountCents <= 0) {
            _uiState.update { it.copy(withdrawError = "Amount must be greater than zero.") }
            return
        }

        _uiState.update { it.copy(isWithdrawing = true, withdrawError = null) }
        viewModelScope.launch {
            try {
                val result = walletRepository.withdrawSol(amountCents, address)
                _uiState.update {
                    it.copy(
                        isWithdrawing = false,
                        lastWithdrawalId = result.withdrawalId,
                        lastWithdrawalStatus = result.status,
                        // Clear form on success
                        withdrawAmountInput = "",
                        withdrawAmountCents = 0L,
                        withdrawAddress = "",
                    )
                }
            } catch (t: Throwable) {
                if (t is CancellationException) throw t
                _uiState.update {
                    it.copy(
                        isWithdrawing = false,
                        withdrawError = AuthErrorMessages.from(t, "Withdrawal failed."),
                    )
                }
            }
        }
    }

    // --- Transaction history ---

    /** Filter transactions by type ("deposit" or "withdraw"). */
    fun filterByType(type: String) {
        _uiState.update { it.copy(activeFilter = type) }
        loadTxHistory(type = type)
    }

    /** Clear the active type filter and reload all transactions. */
    fun clearFilter() {
        _uiState.update { it.copy(activeFilter = null) }
        loadTxHistory()
    }

    // --- Private ---

    private fun loadBalance() {
        viewModelScope.launch {
            try {
                val wallet = walletRepository.getBalance()
                _uiState.update {
                    it.copy(
                        isLoadingBalance = false,
                        balanceCents = wallet.balance,
                        balanceFormatted = formatCentsAsDollars(wallet.balance),
                        currency = wallet.currency,
                    )
                }
            } catch (t: Throwable) {
                if (t is CancellationException) throw t
                _uiState.update {
                    it.copy(
                        isLoadingBalance = false,
                        balanceError = AuthErrorMessages.from(t, "Failed to load wallet balance."),
                    )
                }
            }
        }
    }

    private fun loadTxHistory(type: String? = null) {
        viewModelScope.launch {
            try {
                val result = walletRepository.getTxHistory(type = type)
                _uiState.update {
                    it.copy(
                        isLoadingHistory = false,
                        transactions = result.transactions,
                        historyTotal = result.total,
                        historyPage = result.page,
                    )
                }
            } catch (t: Throwable) {
                if (t is CancellationException) throw t
                _uiState.update {
                    it.copy(
                        isLoadingHistory = false,
                        historyError = AuthErrorMessages.from(t, "Failed to load transaction history."),
                    )
                }
            }
        }
    }

    companion object {
        /** Parse a dollar string like "123.45" to cents (12345L). Returns 0L on invalid input. */
        fun parseDollarsToCents(input: String): Long {
            val trimmed = input.trim()
            if (trimmed.isEmpty()) return 0L
            return try {
                val value = trimmed.toDouble()
                if (value <= 0) 0L else (value * 100).toLong()
            } catch (_: NumberFormatException) {
                0L
            }
        }

        /** Format integer cents as "$X.XX". */
        fun formatCentsAsDollars(cents: Long): String {
            val dollars = cents / 100
            val remainder = cents % 100
            return "$${dollars}.${remainder.toString().padStart(2, '0')}"
        }
    }
}
