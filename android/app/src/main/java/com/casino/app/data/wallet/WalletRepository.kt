package com.casino.app.data.wallet

import com.casino.app.data.api.WalletApi
import com.casino.app.data.api.dto.CryptoWithdrawRequest
import com.casino.app.data.api.dto.CryptoWithdrawResponse
import com.casino.app.data.api.dto.SolanaTxDto
import com.casino.app.data.api.dto.SolDepositResponse
import com.casino.app.data.api.dto.TxHistoryResultDto
import com.casino.app.data.api.dto.UsdcDepositResponse
import com.casino.app.data.api.dto.WalletDto

/**
 * Repository for the wallet API endpoints (design tasks 5.1–5.3).
 *
 * Wraps [WalletApi] and unwraps the [ApiEnvelope][com.casino.app.data.api.dto.ApiEnvelope]
 * on success; non-2xx responses propagate as [retrofit2.HttpException].
 *
 * Design decisions:
 * - All monetary amounts are in integer cents (e.g. 15000 = $150.00).
 * - The backend resolves addresses at deposit time via HD derivation (PR 2).
 * - Withdrawals go through a KYC-gated request flow (PR 3).
 */
class WalletRepository(private val walletApi: WalletApi) {

    /** GET /wallet/balance — returns the authenticated user's wallet. */
    suspend fun getBalance(): WalletDto {
        return walletApi.getBalance().data ?: error("wallet balance succeeded without data")
    }

    /** POST /wallet/deposit/sol — get a SOL deposit address and QR data. */
    suspend fun depositSol(): SolDepositResponse {
        return walletApi.depositSol().data ?: error("SOL deposit succeeded without data")
    }

    /** POST /wallet/deposit/usdc — get a USDC ATA deposit address and QR data. */
    suspend fun depositUsdc(): UsdcDepositResponse {
        return walletApi.depositUsdc().data ?: error("USDC deposit succeeded without data")
    }

    /** POST /wallet/withdraw/sol — request a SOL withdrawal. */
    suspend fun withdrawSol(amount: Long, toAddress: String): CryptoWithdrawResponse {
        return walletApi.withdrawSol(CryptoWithdrawRequest(amount, toAddress)).data
            ?: error("SOL withdrawal succeeded without data")
    }

    /** POST /wallet/withdraw/usdc — request a USDC withdrawal. */
    suspend fun withdrawUsdc(amount: Long, toAddress: String): CryptoWithdrawResponse {
        return walletApi.withdrawUsdc(CryptoWithdrawRequest(amount, toAddress)).data
            ?: error("USDC withdrawal succeeded without data")
    }

    /** GET /wallet/tx-history — paginated Solana transaction history. */
    suspend fun getTxHistory(
        type: String? = null,
        after: String? = null,
        before: String? = null,
        page: Int? = null,
        limit: Int? = null,
    ): TxHistoryResultDto {
        return walletApi.getTxHistory(type, after, before, page, limit).data
            ?: error("tx-history succeeded without data")
    }
}
