package com.casino.app.ui.screens.wallet

import com.casino.app.MainDispatcherRule
import com.casino.app.data.api.dto.CryptoWithdrawResponse
import com.casino.app.data.api.dto.SolanaTxDto
import com.casino.app.data.api.dto.SolDepositResponse
import com.casino.app.data.api.dto.TxHistoryResultDto
import com.casino.app.data.api.dto.UsdcDepositResponse
import com.casino.app.data.api.dto.WalletDto
import com.casino.app.data.wallet.WalletRepository
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.mockk
import java.io.IOException
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.runTest
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.ResponseBody.Companion.toResponseBody
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import retrofit2.HttpException
import retrofit2.Response

/**
 * RED (5.4): WalletViewModel unit tests.
 *
 * Spec scenarios covered:
 * - Load balance on init → balance renders with cents → dollars conversion
 * - Load balance error → error state
 * - Get deposit address (SOL) → address + QR data
 * - Withdraw SOL → success with withdrawal ID and status
 * - Withdraw with empty address → validation error
 * - Withdraw with zero/negative amount → validation error
 * - Load transaction history → list renders
 * - Load transaction history with error → error state
 * - Load transaction history with filter → filter params forwarded
 */
@OptIn(ExperimentalCoroutinesApi::class)
class WalletViewModelTest {

    @get:Rule
    val mainDispatcherRule = MainDispatcherRule()

    private lateinit var repository: WalletRepository
    private lateinit var viewModel: WalletViewModel

    private val testWallet = WalletDto(
        id = "w-1",
        userId = "u-1",
        balance = 15000L,
        currency = "USD",
        version = 1,
        createdAt = "2026-08-01T10:00:00Z",
        updatedAt = "2026-08-10T12:00:00Z",
    )

    private val testTx = SolanaTxDto(
        id = "stx-1",
        walletId = "w-1",
        txHash = "abc123def456",
        type = "deposit",
        amount = 5000L,
        fromAddress = "SENDER111",
        toAddress = "RECEIVER222",
        confirmations = 32,
        networkFee = 500L,
        usdValue = 5000L,
        createdAt = "2026-08-10T12:00:00Z",
    )

    private val testTxHistory = TxHistoryResultDto(
        transactions = listOf(testTx),
        total = 1,
        page = 1,
        limit = 50,
    )

    @Before
    fun setup() {
        repository = mockk()
    }

    private fun initViewModel() {
        viewModel = WalletViewModel(repository)
    }

    // --- Balance tests ---

    @Test
    fun `initial load sets balance on success`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getBalance() } returns testWallet
            coEvery { repository.getTxHistory(any(), any(), any(), any(), any()) } returns testTxHistory

            initViewModel()
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isLoadingBalance)
            assertNull(state.balanceError)
            assertEquals(15000L, state.balanceCents)
            assertEquals("$150.00", state.balanceFormatted)
            assertEquals("USD", state.currency)
        }

    @Test
    fun `load balance error sets error state`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getBalance() } throws IOException("connection reset")
            coEvery { repository.getTxHistory(any(), any(), any(), any(), any()) } returns testTxHistory

            initViewModel()
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isLoadingBalance)
            assertNotNull(state.balanceError)
            assertTrue(state.balanceError!!.contains("Network"))
        }

    // --- Deposit tests ---

    @Test
    fun `getSolDeposit returns address and qr data`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getBalance() } returns testWallet
            coEvery { repository.getTxHistory(any(), any(), any(), any(), any()) } returns testTxHistory
            coEvery { repository.depositSol() } returns SolDepositResponse(
                address = "SOL_abc123",
                qrData = "solana:SOL_abc123",
            )

            initViewModel()
            advanceUntilIdle()

            viewModel.getSolDeposit()
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isDepositing)
            assertNull(state.depositError)
            assertEquals("SOL_abc123", state.depositAddress)
            assertEquals("solana:SOL_abc123", state.depositQrData)
        }

    @Test
    fun `getSolDeposit error sets error state`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getBalance() } returns testWallet
            coEvery { repository.getTxHistory(any(), any(), any(), any(), any()) } returns testTxHistory
            coEvery { repository.depositSol() } throws IOException("timeout")

            initViewModel()
            advanceUntilIdle()

            viewModel.getSolDeposit()
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isDepositing)
            assertNotNull(state.depositError)
        }

    // --- Withdrawal tests ---

    @Test
    fun `withdraw SOL with valid data succeeds`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getBalance() } returns testWallet
            coEvery { repository.getTxHistory(any(), any(), any(), any(), any()) } returns testTxHistory
            coEvery { repository.withdrawSol(any(), any()) } returns CryptoWithdrawResponse(
                withdrawalId = "wd-1",
                status = "pending",
            )

            initViewModel()
            advanceUntilIdle()

            viewModel.onWithdrawAmountChange("100.00")
            viewModel.onWithdrawAddressChange("RECEIVER333")
            viewModel.withdrawSol()
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isWithdrawing)
            assertNull(state.withdrawError)
            assertEquals("wd-1", state.lastWithdrawalId)
            assertEquals("pending", state.lastWithdrawalStatus)
        }

    @Test
    fun `withdraw with empty address shows validation error`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getBalance() } returns testWallet
            coEvery { repository.getTxHistory(any(), any(), any(), any(), any()) } returns testTxHistory

            initViewModel()
            advanceUntilIdle()

            viewModel.onWithdrawAmountChange("100.00")
            viewModel.onWithdrawAddressChange("")
            viewModel.withdrawSol()
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isWithdrawing)
            assertNotNull(state.withdrawError)
            assertTrue(state.withdrawError!!.contains("address"))
        }

    @Test
    fun `withdraw with zero amount shows validation error`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getBalance() } returns testWallet
            coEvery { repository.getTxHistory(any(), any(), any(), any(), any()) } returns testTxHistory

            initViewModel()
            advanceUntilIdle()

            viewModel.onWithdrawAmountChange("0")
            viewModel.onWithdrawAddressChange("RECEIVER333")
            viewModel.withdrawSol()
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isWithdrawing)
            assertNotNull(state.withdrawError)
            assertTrue(state.withdrawError!!.contains("Amount"))
        }

    @Test
    fun `withdraw with negative amount shows validation error`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getBalance() } returns testWallet
            coEvery { repository.getTxHistory(any(), any(), any(), any(), any()) } returns testTxHistory

            initViewModel()
            advanceUntilIdle()

            viewModel.onWithdrawAmountChange("-50")
            viewModel.onWithdrawAddressChange("RECEIVER333")
            viewModel.withdrawSol()
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isWithdrawing)
            assertNotNull(state.withdrawError)
        }

    @Test
    fun `withdraw SOL network error sets error state`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getBalance() } returns testWallet
            coEvery { repository.getTxHistory(any(), any(), any(), any(), any()) } returns testTxHistory
            coEvery { repository.withdrawSol(any(), any()) } throws IOException("connection refused")

            initViewModel()
            advanceUntilIdle()

            viewModel.onWithdrawAmountChange("50.00")
            viewModel.onWithdrawAddressChange("RECEIVER333")
            viewModel.withdrawSol()
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isWithdrawing)
            assertNotNull(state.withdrawError)
        }

    @Test
    fun `withdraw SOL server error surfaces backend message`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getBalance() } returns testWallet
            coEvery { repository.getTxHistory(any(), any(), any(), any(), any()) } returns testTxHistory
            coEvery { repository.withdrawSol(any(), any()) } throws HttpException(
                Response.error<Any>(400, """{"success":false,"error":"daily limit exceeded"}""".toResponseBody("application/json".toMediaType()))
            )

            initViewModel()
            advanceUntilIdle()

            viewModel.onWithdrawAmountChange("500.00")
            viewModel.onWithdrawAddressChange("RECEIVER333")
            viewModel.withdrawSol()
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isWithdrawing)
            assertEquals("daily limit exceeded", state.withdrawError)
        }

    // --- Transaction history tests ---

    @Test
    fun `load tx history on init succeeds`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getBalance() } returns testWallet
            coEvery { repository.getTxHistory(any(), any(), any(), any(), any()) } returns testTxHistory

            initViewModel()
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isLoadingHistory)
            assertNull(state.historyError)
            assertEquals(1, state.transactions.size)
            assertEquals("abc123def456", state.transactions[0].txHash)
            assertEquals("deposit", state.transactions[0].type)
            assertEquals(5000L, state.transactions[0].amount)
        }

    @Test
    fun `load tx history error sets error state`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getBalance() } returns testWallet
            coEvery { repository.getTxHistory(any(), any(), any(), any(), any()) } throws IOException("timeout")

            initViewModel()
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isLoadingHistory)
            assertNotNull(state.historyError)
        }

    @Test
    fun `load tx history empty returns empty list`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getBalance() } returns testWallet
            coEvery { repository.getTxHistory(any(), any(), any(), any(), any()) } returns TxHistoryResultDto(
                transactions = emptyList(),
                total = 0,
                page = 1,
                limit = 50,
            )

            initViewModel()
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isLoadingHistory)
            assertNull(state.historyError)
            assertEquals(0, state.transactions.size)
        }

    @Test
    fun `filter tx history by type forwards filter to repository`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getBalance() } returns testWallet
            coEvery { repository.getTxHistory(any(), any(), any(), any(), any()) } returns testTxHistory

            initViewModel()
            advanceUntilIdle()

            viewModel.filterByType("deposit")
            advanceUntilIdle()

            coVerify { repository.getTxHistory(type = "deposit", after = any(), before = any(), page = any(), limit = any()) }
            assertEquals("deposit", viewModel.uiState.value.activeFilter)
        }

    @Test
    fun `clear filter loads all transactions`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getBalance() } returns testWallet
            coEvery { repository.getTxHistory(any(), any(), any(), any(), any()) } returns testTxHistory

            initViewModel()
            advanceUntilIdle()

            viewModel.filterByType("deposit")
            advanceUntilIdle()

            viewModel.clearFilter()
            advanceUntilIdle()

            assertEquals(null, viewModel.uiState.value.activeFilter)
        }

    // --- Withdrawal amount parsing tests ---

    @Test
    fun `amount parsing converts dollars to cents`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getBalance() } returns testWallet
            coEvery { repository.getTxHistory(any(), any(), any(), any(), any()) } returns testTxHistory

            initViewModel()
            advanceUntilIdle()

            viewModel.onWithdrawAmountChange("123.45")
            assertEquals(12345L, viewModel.uiState.value.withdrawAmountCents)
        }

    @Test
    fun `amount parsing handles whole dollars`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getBalance() } returns testWallet
            coEvery { repository.getTxHistory(any(), any(), any(), any(), any()) } returns testTxHistory

            initViewModel()
            advanceUntilIdle()

            viewModel.onWithdrawAmountChange("100")
            assertEquals(10000L, viewModel.uiState.value.withdrawAmountCents)
        }

    @Test
    fun `amount parsing handles invalid input as zero`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getBalance() } returns testWallet
            coEvery { repository.getTxHistory(any(), any(), any(), any(), any()) } returns testTxHistory

            initViewModel()
            advanceUntilIdle()

            viewModel.onWithdrawAmountChange("abc")
            assertEquals(0L, viewModel.uiState.value.withdrawAmountCents)
        }
}
