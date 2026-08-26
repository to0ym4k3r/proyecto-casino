package com.casino.app.ui.bonus

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
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.casino.app.data.api.dto.BonusDefinitionDto
import com.casino.app.ui.util.formatCentsAsDollars
import com.casino.app.ui.theme.AccentGold
import com.casino.app.ui.theme.AccentGreen
import com.casino.app.ui.theme.AccentRed
import com.casino.app.ui.theme.BgDeep
import com.casino.app.ui.theme.BgElevated
import com.casino.app.ui.theme.BgSurface
import com.casino.app.ui.theme.TextMuted
import com.casino.app.ui.theme.TextPrimary
import com.casino.app.ui.theme.TextSecondary

/**
 * Screen displaying available claimable bonuses (task 6.6).
 *
 * Shows a LazyColumn of bonus cards with claim button per bonus.
 * Includes a confirmation dialog before claiming.
 */
@Composable
fun BonusListScreen(
    viewModel: BonusViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsStateWithLifecycle()
    var showConfirmDialog by remember { mutableStateOf<BonusDefinitionDto?>(null) }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(BgDeep)
    ) {
        // Header
        Text(
            text = "BONUSES",
            color = AccentGreen,
            fontSize = 28.sp,
            fontWeight = FontWeight.ExtraBold,
            modifier = Modifier.padding(horizontal = 16.dp, vertical = 12.dp)
        )

        // Content
        when {
            uiState.isLoadingAvailable -> {
                Box(
                    modifier = Modifier.fillMaxSize(),
                    contentAlignment = Alignment.Center,
                ) {
                    CircularProgressIndicator(color = AccentGreen, strokeWidth = 2.dp)
                }
            }
            uiState.availableError != null -> {
                Box(
                    modifier = Modifier.fillMaxSize(),
                    contentAlignment = Alignment.Center,
                ) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Text(text = uiState.availableError!!, color = AccentRed, fontSize = 14.sp)
                        Spacer(modifier = Modifier.height(12.dp))
                        Button(
                            onClick = { viewModel.loadAvailable() },
                            colors = ButtonDefaults.buttonColors(containerColor = AccentGreen),
                            shape = RoundedCornerShape(8.dp),
                        ) {
                            Text("Retry", color = BgDeep, fontWeight = FontWeight.SemiBold)
                        }
                    }
                }
            }
            uiState.availableBonuses.isEmpty() -> {
                Box(
                    modifier = Modifier.fillMaxSize(),
                    contentAlignment = Alignment.Center,
                ) {
                    Text(text = "No bonuses available right now", color = TextMuted, fontSize = 14.sp)
                }
            }
            else -> {
                LazyColumn(
                    modifier = Modifier.padding(horizontal = 16.dp),
                    verticalArrangement = Arrangement.spacedBy(12.dp),
                ) {
                    items(uiState.availableBonuses, key = { it.id }) { bonus ->
                        BonusDefinitionCard(
                            bonus = bonus,
                            isClaiming = uiState.isClaiming,
                            onClaimClick = { showConfirmDialog = bonus },
                        )
                    }
                }
            }
        }

        // Claim error
        if (uiState.claimError != null) {
            Text(
                text = uiState.claimError!!,
                color = AccentRed,
                fontSize = 13.sp,
                modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
            )
        }
    }

    // Confirmation dialog
    showConfirmDialog?.let { bonus ->
        ClaimConfirmDialog(
            bonus = bonus,
            onConfirm = {
                viewModel.claimBonus(bonus.id)
                showConfirmDialog = null
            },
            onDismiss = { showConfirmDialog = null },
        )
    }
}

@Composable
private fun BonusDefinitionCard(
    bonus: BonusDefinitionDto,
    isClaiming: Boolean,
    onClaimClick: () -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(12.dp))
            .background(BgSurface)
            .padding(16.dp),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                text = bonus.name,
                color = TextPrimary,
                fontSize = 18.sp,
                fontWeight = FontWeight.Bold,
            )
            Text(
                text = formatBonusType(bonus.code),
                color = AccentGold,
                fontSize = 12.sp,
                fontWeight = FontWeight.SemiBold,
            )
        }

        Spacer(modifier = Modifier.height(8.dp))

        Text(
            text = bonus.description,
            color = TextSecondary,
            fontSize = 14.sp,
        )

        Spacer(modifier = Modifier.height(12.dp))

        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
        ) {
            Column {
                Text(text = "Max Bonus", color = TextMuted, fontSize = 12.sp)
                Text(
                    text = formatCentsAsDollars(bonus.maxBonusAmountCents),
                    color = TextPrimary,
                    fontSize = 14.sp,
                    fontWeight = FontWeight.SemiBold,
                )
            }
            Column {
                Text(text = "Wagering", color = TextMuted, fontSize = 12.sp)
                Text(
                    text = "${bonus.wageringMultiplier}x",
                    color = TextPrimary,
                    fontSize = 14.sp,
                    fontWeight = FontWeight.SemiBold,
                )
            }
            Column {
                Text(text = "Expires", color = TextMuted, fontSize = 12.sp)
                Text(
                    text = "${bonus.expiryDays} days",
                    color = TextPrimary,
                    fontSize = 14.sp,
                    fontWeight = FontWeight.SemiBold,
                )
            }
            if (bonus.minDepositCents > 0) {
                Column {
                    Text(text = "Min Deposit", color = TextMuted, fontSize = 12.sp)
                    Text(
                        text = formatCentsAsDollars(bonus.minDepositCents),
                        color = TextPrimary,
                        fontSize = 14.sp,
                        fontWeight = FontWeight.SemiBold,
                    )
                }
            }
        }

        Spacer(modifier = Modifier.height(16.dp))

        Button(
            onClick = onClaimClick,
            enabled = !isClaiming,
            colors = ButtonDefaults.buttonColors(containerColor = AccentGreen),
            shape = RoundedCornerShape(8.dp),
            modifier = Modifier.fillMaxWidth(),
        ) {
            if (isClaiming) {
                CircularProgressIndicator(
                    color = BgDeep,
                    strokeWidth = 2.dp,
                    modifier = Modifier.size(18.dp),
                )
                Spacer(modifier = Modifier.width(8.dp))
            }
            Text(
                text = if (isClaiming) "Claiming..." else "Claim Bonus",
                color = BgDeep,
                fontWeight = FontWeight.SemiBold,
            )
        }
    }
}

@Composable
private fun ClaimConfirmDialog(
    bonus: BonusDefinitionDto,
    onConfirm: () -> Unit,
    onDismiss: () -> Unit,
) {
    AlertDialog(
        onDismissRequest = onDismiss,
        containerColor = BgElevated,
        title = {
            Text(
                text = "Claim ${bonus.name}?",
                color = TextPrimary,
                fontWeight = FontWeight.Bold,
            )
        },
        text = {
            Column {
                Text(
                    text = bonus.description,
                    color = TextSecondary,
                    fontSize = 14.sp,
                )
                Spacer(modifier = Modifier.height(12.dp))
                Text(
                    text = "Wagering requirement: ${bonus.wageringMultiplier}x",
                    color = TextSecondary,
                    fontSize = 13.sp,
                )
                Text(
                    text = "Expires in: ${bonus.expiryDays} days",
                    color = TextSecondary,
                    fontSize = 13.sp,
                )
                if (bonus.minDepositCents > 0) {
                    Text(
                        text = "Min deposit: ${formatCentsAsDollars(bonus.minDepositCents)}",
                        color = TextSecondary,
                        fontSize = 13.sp,
                    )
                }
            }
        },
        confirmButton = {
            TextButton(onClick = onConfirm) {
                Text("Confirm", color = AccentGreen, fontWeight = FontWeight.Bold)
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) {
                Text("Cancel", color = TextMuted)
            }
        },
    )
}

private fun formatBonusType(code: String): String = when (code) {
    "welcome" -> "WELCOME"
    "reload" -> "RELOAD"
    "cashback" -> "CASHBACK"
    "free_spins" -> "FREE SPINS"
    else -> code.uppercase()
}
