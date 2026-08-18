package com.casino.app.ui.screens.wallet

import androidx.compose.foundation.background
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
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material.icons.filled.ArrowDownward
import androidx.compose.material.icons.filled.ArrowUpward
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FilterChipDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.casino.app.data.api.dto.SolanaTxDto
import com.casino.app.ui.theme.AccentGreen
import com.casino.app.ui.theme.AccentRed
import com.casino.app.ui.theme.BgDeep
import com.casino.app.ui.theme.BgSurface
import com.casino.app.ui.theme.TextMuted
import com.casino.app.ui.theme.TextPrimary
import com.casino.app.ui.theme.TextSecondary

@Composable
fun TransactionHistoryScreen(
    uiState: WalletUiState,
    onFilterByType: (String) -> Unit,
    onClearFilter: () -> Unit,
    onBack: () -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(BgDeep)
            .padding(16.dp),
    ) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            modifier = Modifier.fillMaxWidth(),
        ) {
            IconButton(onClick = onBack) {
                Icon(
                    imageVector = Icons.Filled.ArrowBack,
                    contentDescription = "Back",
                    tint = TextSecondary,
                )
            }
            Text(
                text = "Transaction History",
                color = TextPrimary,
                fontSize = 20.sp,
                fontWeight = FontWeight.Bold,
            )
        }

        Spacer(modifier = Modifier.height(12.dp))

        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            val filters = listOf(null to "All", "deposit" to "Deposits", "withdraw" to "Withdrawals")
            filters.forEach { (type, label) ->
                FilterChip(
                    selected = uiState.activeFilter == type,
                    onClick = {
                        if (type == null) onClearFilter() else onFilterByType(type)
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
