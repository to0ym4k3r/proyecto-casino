package com.casino.app.ui.screens.wallet

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material.icons.filled.ArrowDownward
import androidx.compose.material.icons.filled.ArrowUpward
import androidx.compose.material.icons.filled.ContentCopy
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FilterChipDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.casino.app.data.api.dto.SolanaTxDto
import com.casino.app.ui.theme.AccentGreen
import com.casino.app.ui.theme.AccentRed
import com.casino.app.ui.theme.BgDeep
import com.casino.app.ui.theme.BgElevated
import com.casino.app.ui.theme.BgPrimary
import com.casino.app.ui.theme.BgSurface
import com.casino.app.ui.theme.TextMuted
import com.casino.app.ui.theme.TextPrimary
import com.casino.app.ui.theme.TextSecondary

/**
 * Wallet screen composable with tab navigation for balance, deposit, withdraw,
 * and transaction history.
 *
 * Follows the project's composable patterns (LobbyScreen, ProfileScreen):
 * hiltViewModel() injection, collectAsStateWithLifecycle, theme colors.
 */
@Composable
fun WalletScreen(
    viewModel: WalletViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsStateWithLifecycle()
    var selectedTab by remember { mutableStateOf(WalletTab.Balance) }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(BgDeep)
    ) {
        // Header
        Text(
            text = "WALLET",
            color = AccentGreen,
            fontSize = 28.sp,
            fontWeight = FontWeight.ExtraBold,
            modifier = Modifier.padding(horizontal = 16.dp, vertical = 12.dp)
        )

        // Tab row
        WalletTabRow(selectedTab = selectedTab, onTabSelected = { selectedTab = it })

        // Tab content
        when (selectedTab) {
            WalletTab.Balance -> BalanceTab(uiState)
            WalletTab.Deposit -> DepositTab(uiState, viewModel)
            WalletTab.Withdraw -> WithdrawTab(uiState, viewModel)
            WalletTab.History -> HistoryTab(uiState, viewModel)
        }
    }
}

// --- Tab definitions ---

private enum class WalletTab(val label: String) {
    Balance("Balance"),
    Deposit("Deposit"),
    Withdraw("Withdraw"),
    History("History"),
}

@Composable
private fun WalletTabRow(
    selectedTab: WalletTab,
    onTabSelected: (WalletTab) -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 8.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        WalletTab.entries.forEach { tab ->
            FilterChip(
                selected = selectedTab == tab,
                onClick = { onTabSelected(tab) },
                label = {
                    Text(
                        text = tab.label,
                        fontSize = 13.sp,
                        fontWeight = FontWeight.SemiBold,
                    )
                },
                colors = FilterChipDefaults.filterChipColors(
                    selectedContainerColor = AccentGreen,
                    selectedLabelColor = BgDeep,
                    containerColor = BgSurface,
                    labelColor = TextSecondary,
                ),
                shape = RoundedCornerShape(6.dp),
            )
        }
    }
}

// --- Balance tab ---

@Composable
private fun BalanceTab(uiState: WalletUiState) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(16.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        if (uiState.isLoadingBalance) {
            Spacer(modifier = Modifier.height(48.dp))
            CircularProgressIndicator(color = AccentGreen, strokeWidth = 2.dp)
        } else if (uiState.balanceError != null) {
            Spacer(modifier = Modifier.height(24.dp))
            Text(text = uiState.balanceError!!, color = AccentRed, fontSize = 14.sp)
        } else {
            // Balance card
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .clip(RoundedCornerShape(12.dp))
                    .background(
                        Brush.verticalGradient(
                            colors = listOf(BgPrimary, BgElevated),
                        ),
                    )
                    .padding(24.dp),
                contentAlignment = Alignment.Center,
            ) {
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    Text(
                        text = "Total Balance",
                        color = TextSecondary,
                        fontSize = 14.sp,
                    )
                    Spacer(modifier = Modifier.height(8.dp))
                    Text(
                        text = uiState.balanceFormatted,
                        color = TextPrimary,
                        fontSize = 36.sp,
                        fontWeight = FontWeight.Bold,
                    )
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(
                        text = uiState.currency,
                        color = TextMuted,
                        fontSize = 13.sp,
                    )
                }
            }
        }
    }
}

// --- Deposit tab ---

@Composable
private fun DepositTab(uiState: WalletUiState, viewModel: WalletViewModel) {
    val clipboard = LocalClipboardManager.current

    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(16.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Spacer(modifier = Modifier.height(16.dp))

        // SOL deposit button
        Button(
            onClick = { viewModel.getSolDeposit() },
            enabled = !uiState.isDepositing,
            colors = ButtonDefaults.buttonColors(containerColor = AccentGreen),
            shape = RoundedCornerShape(8.dp),
            modifier = Modifier.fillMaxWidth(),
        ) {
            if (uiState.isDepositing) {
                CircularProgressIndicator(
                    color = BgDeep,
                    strokeWidth = 2.dp,
                    modifier = Modifier.size(18.dp),
                )
                Spacer(modifier = Modifier.width(8.dp))
            }
            Text(
                text = if (uiState.isDepositing) "Generating..." else "Get SOL Deposit Address",
                color = BgDeep,
                fontWeight = FontWeight.SemiBold,
            )
        }

        if (uiState.depositError != null) {
            Spacer(modifier = Modifier.height(8.dp))
            Text(text = uiState.depositError!!, color = AccentRed, fontSize = 13.sp)
        }

        if (uiState.depositAddress != null) {
            Spacer(modifier = Modifier.height(24.dp))

            // Address card
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .clip(RoundedCornerShape(12.dp))
                    .background(BgSurface)
                    .padding(16.dp),
            ) {
                Column {
                    Text(text = "Your SOL Address", color = TextSecondary, fontSize = 13.sp)
                    Spacer(modifier = Modifier.height(8.dp))
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Text(
                            text = uiState.depositAddress!!,
                            color = TextPrimary,
                            fontSize = 14.sp,
                            fontWeight = FontWeight.Medium,
                            modifier = Modifier.weight(1f),
                        )
                        IconButton(onClick = {
                            clipboard.setText(AnnotatedString(uiState.depositAddress!!))
                        }) {
                            Icon(
                                imageVector = Icons.Filled.ContentCopy,
                                contentDescription = "Copy address",
                                tint = AccentGreen,
                                modifier = Modifier.size(20.dp),
                            )
                        }
                    }
                    if (uiState.depositQrData != null) {
                        Spacer(modifier = Modifier.height(12.dp))
                        Text(text = "Scan QR to deposit", color = TextMuted, fontSize = 12.sp)
                        // QR code would be rendered here with a QR library
                    }
                }
            }
        }
    }
}

// --- Withdraw tab ---

@Composable
private fun WithdrawTab(uiState: WalletUiState, viewModel: WalletViewModel) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(16.dp),
    ) {
        Spacer(modifier = Modifier.height(8.dp))

        // Amount input
        OutlinedTextField(
            value = uiState.withdrawAmountInput,
            onValueChange = { viewModel.onWithdrawAmountChange(it) },
            label = { Text("Amount (USD)") },
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
            modifier = Modifier.fillMaxWidth(),
            singleLine = true,
            colors = OutlinedTextFieldDefaults.colors(
                focusedBorderColor = AccentGreen,
                unfocusedBorderColor = BgSurface,
                focusedContainerColor = BgSurface,
                unfocusedContainerColor = BgSurface,
                cursorColor = AccentGreen,
                focusedTextColor = TextPrimary,
                unfocusedTextColor = TextPrimary,
            ),
            shape = RoundedCornerShape(8.dp),
        )

        Spacer(modifier = Modifier.height(12.dp))

        // Address input
        OutlinedTextField(
            value = uiState.withdrawAddress,
            onValueChange = { viewModel.onWithdrawAddressChange(it) },
            label = { Text("Recipient SOL Address") },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true,
            colors = OutlinedTextFieldDefaults.colors(
                focusedBorderColor = AccentGreen,
                unfocusedBorderColor = BgSurface,
                focusedContainerColor = BgSurface,
                unfocusedContainerColor = BgSurface,
                cursorColor = AccentGreen,
                focusedTextColor = TextPrimary,
                unfocusedTextColor = TextPrimary,
            ),
            shape = RoundedCornerShape(8.dp),
        )

        if (uiState.withdrawError != null) {
            Spacer(modifier = Modifier.height(8.dp))
            Text(text = uiState.withdrawError!!, color = AccentRed, fontSize = 13.sp)
        }

        if (uiState.lastWithdrawalId != null) {
            Spacer(modifier = Modifier.height(8.dp))
            Text(
                text = "Withdrawal ${uiState.lastWithdrawalStatus}: ${uiState.lastWithdrawalId}",
                color = AccentGreen,
                fontSize = 13.sp,
            )
        }

        Spacer(modifier = Modifier.height(20.dp))

        Button(
            onClick = { viewModel.withdrawSol() },
            enabled = !uiState.isWithdrawing && uiState.withdrawAmountCents > 0 && uiState.withdrawAddress.isNotBlank(),
            colors = ButtonDefaults.buttonColors(containerColor = AccentGreen),
            shape = RoundedCornerShape(8.dp),
            modifier = Modifier.fillMaxWidth(),
        ) {
            if (uiState.isWithdrawing) {
                CircularProgressIndicator(
                    color = BgDeep,
                    strokeWidth = 2.dp,
                    modifier = Modifier.size(18.dp),
                )
                Spacer(modifier = Modifier.width(8.dp))
            }
            Text(
                text = if (uiState.isWithdrawing) "Processing..." else "Withdraw SOL",
                color = BgDeep,
                fontWeight = FontWeight.SemiBold,
            )
        }
    }
}

// --- History tab ---

@Composable
private fun HistoryTab(uiState: WalletUiState, viewModel: WalletViewModel) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(16.dp),
    ) {
        // Filter chips
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            val filters = listOf(null to "All", "deposit" to "Deposits", "withdraw" to "Withdrawals")
            filters.forEach { (type, label) ->
                FilterChip(
                    selected = uiState.activeFilter == type,
                    onClick = {
                        if (type == null) viewModel.clearFilter() else viewModel.filterByType(type)
                    },
                    label = { Text(label, fontSize = 12.sp, fontWeight = FontWeight.SemiBold) },
                    colors = FilterChipDefaults.filterChipColors(
                        selectedContainerColor = AccentGreen,
                        selectedLabelColor = BgDeep,
                        containerColor = BgSurface,
                        labelColor = TextSecondary,
                    ),
                    shape = RoundedCornerShape(6.dp),
                )
            }
        }

        Spacer(modifier = Modifier.height(12.dp))

        if (uiState.isLoadingHistory) {
            Box(
                modifier = Modifier.fillMaxSize(),
                contentAlignment = Alignment.Center,
            ) {
                CircularProgressIndicator(color = AccentGreen, strokeWidth = 2.dp)
            }
        } else if (uiState.historyError != null) {
            Text(text = uiState.historyError!!, color = AccentRed, fontSize = 14.sp)
        } else if (uiState.transactions.isEmpty()) {
            Box(
                modifier = Modifier.fillMaxSize(),
                contentAlignment = Alignment.Center,
            ) {
                Text(text = "No transactions yet", color = TextMuted, fontSize = 14.sp)
            }
        } else {
            LazyColumn(
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                items(uiState.transactions, key = { it.id }) { tx ->
                    TxRow(tx)
                }
            }
        }
    }
}

@Composable
private fun TxRow(tx: SolanaTxDto) {
    val isDeposit = tx.type == "deposit"
    val icon = if (isDeposit) Icons.Filled.ArrowDownward else Icons.Filled.ArrowUpward
    val iconColor = if (isDeposit) AccentGreen else AccentRed
    val amountSign = if (isDeposit) "+" else "-"
    val amountFormatted = "$amountSign$${tx.amount / 100}.${(tx.amount % 100).toString().padStart(2, '0')}"

    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(8.dp))
            .background(BgSurface)
            .padding(12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(
            imageVector = icon,
            contentDescription = tx.type,
            tint = iconColor,
            modifier = Modifier.size(24.dp),
        )
        Spacer(modifier = Modifier.width(12.dp))
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = tx.type.replaceFirstChar { it.uppercase() },
                color = TextPrimary,
                fontSize = 14.sp,
                fontWeight = FontWeight.SemiBold,
            )
            Text(
                text = tx.txHash.take(16) + "...",
                color = TextMuted,
                fontSize = 12.sp,
            )
        }
        Text(
            text = amountFormatted,
            color = iconColor,
            fontSize = 14.sp,
            fontWeight = FontWeight.Bold,
        )
    }
}
