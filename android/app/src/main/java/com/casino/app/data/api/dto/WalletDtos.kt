package com.casino.app.data.api.dto

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * Wallet DTOs matching the frozen backend contract (internal/domain/wallet.go).
 *
 * All monetary amounts are in integer cents (e.g. 15000 = $150.00).
 */

/** Wallet balance response from GET /api/v1/wallet/balance. */
@Serializable
data class WalletDto(
    val id: String,
    @SerialName("user_id") val userId: String,
    val balance: Long,
    @SerialName("bonus_balance") val bonusBalance: Long = 0L,
    val currency: String,
    val version: Int,
    @SerialName("created_at") val createdAt: String,
    @SerialName("updated_at") val updatedAt: String,
)

/** SOL deposit address response from POST /api/v1/wallet/deposit/sol. */
@Serializable
data class SolDepositResponse(
    val address: String,
    @SerialName("qr_data") val qrData: String,
)

/** USDC deposit ATA response from POST /api/v1/wallet/deposit/usdc. */
@Serializable
data class UsdcDepositResponse(
    val address: String,
    val ata: String,
    @SerialName("qr_data") val qrData: String,
)

/** Deposit address derived from HD wallet. */
@Serializable
data class DepositAddressDto(
    @SerialName("user_id") val userId: String,
    @SerialName("user_index") val userIndex: Int,
    @SerialName("sol_address") val solAddress: String,
    @SerialName("usdc_ata") val usdcAta: String,
)

/** Crypto withdrawal request body. */
@Serializable
data class CryptoWithdrawRequest(
    val amount: Long,
    @SerialName("to_address") val toAddress: String,
)

/** Crypto withdrawal response from POST /api/v1/wallet/withdraw/{sol,usdc}. */
@Serializable
data class CryptoWithdrawResponse(
    @SerialName("withdrawal_id") val withdrawalId: String,
    val status: String,
)

/** A single on-chain Solana transaction record. */
@Serializable
data class SolanaTxDto(
    val id: String,
    @SerialName("wallet_id") val walletId: String,
    @SerialName("tx_hash") val txHash: String,
    val type: String,
    val amount: Long,
    @SerialName("from_address") val fromAddress: String,
    @SerialName("to_address") val toAddress: String,
    val confirmations: Int,
    @SerialName("network_fee") val networkFee: Long,
    @SerialName("usd_value_at_time") val usdValue: Long,
    @SerialName("created_at") val createdAt: String,
)

/** Paginated transaction history response from GET /api/v1/wallet/tx-history. */
@Serializable
data class TxHistoryResultDto(
    val transactions: List<SolanaTxDto>,
    val total: Int,
    val page: Int,
    val limit: Int,
)
