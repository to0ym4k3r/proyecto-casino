package com.casino.app.data.api

import com.casino.app.data.api.dto.ApiEnvelope
import com.casino.app.data.api.dto.CryptoWithdrawRequest
import com.casino.app.data.api.dto.CryptoWithdrawResponse
import com.casino.app.data.api.dto.DepositAddressDto
import com.casino.app.data.api.dto.SolDepositResponse
import com.casino.app.data.api.dto.TxHistoryResultDto
import com.casino.app.data.api.dto.UsdcDepositResponse
import com.casino.app.data.api.dto.WalletDto
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.Query

/**
 * Retrofit interface for the wallet API (design tasks 5.1–5.3).
 *
 * Covers:
 * - GET /api/v1/wallet/balance — current balance
 * - POST /api/v1/wallet/deposit/sol — get SOL deposit address + QR
 * - POST /api/v1/wallet/deposit/usdc — get USDC ATA deposit address + QR
 * - POST /api/v1/wallet/withdraw/sol — request SOL withdrawal
 * - POST /api/v1/wallet/withdraw/usdc — request USDC withdrawal
 * - GET /api/v1/wallet/tx-history — paginated transaction history
 */
interface WalletApi {

    /** GET /api/v1/wallet/balance — returns the wallet balance in integer cents. */
    @GET("wallet/balance")
    suspend fun getBalance(): ApiEnvelope<WalletDto>

    /** POST /api/v1/wallet/deposit/sol — returns a SOL deposit address and QR data. */
    @POST("wallet/deposit/sol")
    suspend fun depositSol(): ApiEnvelope<SolDepositResponse>

    /** POST /api/v1/wallet/deposit/usdc — returns a USDC ATA deposit address and QR data. */
    @POST("wallet/deposit/usdc")
    suspend fun depositUsdc(): ApiEnvelope<UsdcDepositResponse>

    /** POST /api/v1/wallet/withdraw/sol — request a SOL withdrawal. */
    @POST("wallet/withdraw/sol")
    suspend fun withdrawSol(@Body body: CryptoWithdrawRequest): ApiEnvelope<CryptoWithdrawResponse>

    /** POST /api/v1/wallet/withdraw/usdc — request a USDC withdrawal. */
    @POST("wallet/withdraw/usdc")
    suspend fun withdrawUsdc(@Body body: CryptoWithdrawRequest): ApiEnvelope<CryptoWithdrawResponse>

    /**
     * GET /api/v1/wallet/tx-history — paginated Solana transaction history.
     *
     * @param type   filter by "deposit" or "withdraw"
     * @param after  RFC 3339 lower bound (inclusive)
     * @param before RFC 3339 upper bound (inclusive)
     * @param page   1-indexed page number
     * @param limit  max results per page (default 50)
     */
    @GET("wallet/tx-history")
    suspend fun getTxHistory(
        @Query("type") type: String? = null,
        @Query("after") after: String? = null,
        @Query("before") before: String? = null,
        @Query("page") page: Int? = null,
        @Query("limit") limit: Int? = null,
    ): ApiEnvelope<TxHistoryResultDto>
}
