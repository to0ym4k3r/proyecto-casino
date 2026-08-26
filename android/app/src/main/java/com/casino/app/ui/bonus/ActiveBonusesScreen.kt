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
import androidx.compose.material3.LinearProgressIndicator
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
import com.casino.app.data.api.dto.ActiveBonusDto
import com.casino.app.ui.util.formatCentsAsDollars
import com.casino.app.ui.theme.AccentGold
import com.casino.app.ui.theme.AccentGreen
import com.casino.app.ui.theme.AccentRed
import com.casino.app.ui.theme.BgDeep
import com.casino.app.ui.theme.BgElevated
import com.casino.app.ui.theme.BgSurface
import com.casino.app.ui.theme.StatusWarning
import com.casino.app.ui.theme.TextMuted
import com.casino.app.ui.theme.TextPrimary
import com.casino.app.ui.theme.TextSecondary

/**
 * Screen displaying active bonus grants with wagering progress (task 6.7).
 *
 * Shows a LazyColumn of active bonus cards with progress bars and forfeit buttons.
 */
@Composable
fun ActiveBonusesScreen(
    viewModel: BonusViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsStateWithLifecycle()
    var showForfeitDialog by remember { mutableStateOf<ActiveBonusDto?>(null) }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(BgDeep)
    ) {
        // Header
        Text(
            text = "ACTIVE BONUSES",
            color = AccentGreen,
            fontSize = 28.sp,
            fontWeight = FontWeight.ExtraBold,
            modifier = Modifier.padding(horizontal = 16.dp, vertical = 12.dp)
        )

        // Content
        when {
            uiState.isLoadingActive -> {
                Box(
                    modifier = Modifier.fillMaxSize(),
                    contentAlignment = Alignment.Center,
                ) {
                    CircularProgressIndicator(color = AccentGreen, strokeWidth = 2.dp)
                }
            }
            uiState.activeError != null -> {
                Box(
                    modifier = Modifier.fillMaxSize(),
                    contentAlignment = Alignment.Center,
                ) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Text(text = uiState.activeError!!, color = AccentRed, fontSize = 14.sp)
                        Spacer(modifier = Modifier.height(12.dp))
                        Button(
                            onClick = { viewModel.loadActive() },
                            colors = ButtonDefaults.buttonColors(containerColor = AccentGreen),
                            shape = RoundedCornerShape(8.dp),
                        ) {
                            Text("Retry", color = BgDeep, fontWeight = FontWeight.SemiBold)
                        }
                    }
                }
            }
            uiState.activeBonuses.isEmpty() -> {
                Box(
                    modifier = Modifier.fillMaxSize(),
                    contentAlignment = Alignment.Center,
                ) {
                    Text(text = "No active bonuses", color = TextMuted, fontSize = 14.sp)
                }
            }
            else -> {
                LazyColumn(
                    modifier = Modifier.padding(horizontal = 16.dp),
                    verticalArrangement = Arrangement.spacedBy(12.dp),
                ) {
                    items(uiState.activeBonuses, key = { it.grantId }) { bonus ->
                        ActiveBonusCard(
                            bonus = bonus,
                            isForfeiting = uiState.isForfeiting,
                            onForfeitClick = { showForfeitDialog = bonus },
                        )
                    }
                }
            }
        }

        // Forfeit error
        if (uiState.forfeitError != null) {
            Text(
                text = uiState.forfeitError!!,
                color = AccentRed,
                fontSize = 13.sp,
                modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
            )
        }
    }

    // Forfeit confirmation dialog
    showForfeitDialog?.let { bonus ->
        ForfeitConfirmDialog(
            bonus = bonus,
            onConfirm = {
                viewModel.forfeitBonus(bonus.grantId)
                showForfeitDialog = null
            },
            onDismiss = { showForfeitDialog = null },
        )
    }
}

@Composable
private fun ActiveBonusCard(
    bonus: ActiveBonusDto,
    isForfeiting: Boolean,
    onForfeitClick: () -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(12.dp))
            .background(BgSurface)
            .padding(16.dp),
    ) {
        // Title row
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
                text = bonus.code.uppercase(),
                color = AccentGold,
                fontSize = 12.sp,
                fontWeight = FontWeight.SemiBold,
            )
        }

        Spacer(modifier = Modifier.height(12.dp))

        // Bonus amount
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
        ) {
            Column {
                Text(text = "Bonus Amount", color = TextMuted, fontSize = 12.sp)
                Text(
                    text = formatCentsAsDollars(bonus.bonusAmountCents),
                    color = AccentGreen,
                    fontSize = 16.sp,
                    fontWeight = FontWeight.Bold,
                )
            }
            Column(horizontalAlignment = Alignment.End) {
                Text(text = "Wagering", color = TextMuted, fontSize = 12.sp)
                Text(
                    text = "${bonus.wageringProgressPct.toInt()}%",
                    color = if (bonus.wageringProgressPct >= 100.0) AccentGreen else TextPrimary,
                    fontSize = 16.sp,
                    fontWeight = FontWeight.Bold,
                )
            }
        }

        Spacer(modifier = Modifier.height(12.dp))

        // Progress bar
        Column {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
            ) {
                Text(
                    text = "${formatCentsAsDollars(bonus.totalWageredCents)} wagered",
                    color = TextSecondary,
                    fontSize = 12.sp,
                )
                Text(
                    text = "of ${formatCentsAsDollars(bonus.wageringRequiredCents)}",
                    color = TextSecondary,
                    fontSize = 12.sp,
                )
            }
            Spacer(modifier = Modifier.height(6.dp))
            LinearProgressIndicator(
                progress = {
                    (bonus.wageringProgressPct / 100.0).coerceIn(0.0, 1.0).toFloat()
                },
                modifier = Modifier
                    .fillMaxWidth()
                    .height(8.dp)
                    .clip(RoundedCornerShape(4.dp)),
                color = when {
                    bonus.wageringProgressPct >= 100.0 -> AccentGreen
                    bonus.wageringProgressPct >= 50.0 -> AccentGold
                    else -> StatusWarning
                },
                trackColor = BgDeep,
            )
        }

        Spacer(modifier = Modifier.height(12.dp))

        // Expiry
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                text = "Expires: ${bonus.expiresAt.take(10)}",
                color = TextMuted,
                fontSize = 12.sp,
            )
            Button(
                onClick = onForfeitClick,
                enabled = !isForfeiting,
                colors = ButtonDefaults.buttonColors(containerColor = AccentRed),
                shape = RoundedCornerShape(6.dp),
            ) {
                if (isForfeiting) {
                    CircularProgressIndicator(
                        color = BgDeep,
                        strokeWidth = 2.dp,
                        modifier = Modifier.size(14.dp),
                    )
                    Spacer(modifier = Modifier.width(6.dp))
                }
                Text(
                    text = if (isForfeiting) "Forfeiting..." else "Forfeit",
                    color = BgDeep,
                    fontSize = 12.sp,
                    fontWeight = FontWeight.SemiBold,
                )
            }
        }
    }
}

@Composable
private fun ForfeitConfirmDialog(
    bonus: ActiveBonusDto,
    onConfirm: () -> Unit,
    onDismiss: () -> Unit,
) {
    AlertDialog(
        onDismissRequest = onDismiss,
        containerColor = BgElevated,
        title = {
            Text(
                text = "Forfeit Bonus?",
                color = AccentRed,
                fontWeight = FontWeight.Bold,
            )
        },
        text = {
            Column {
                Text(
                    text = "Are you sure you want to forfeit your ${bonus.name}?",
                    color = TextSecondary,
                    fontSize = 14.sp,
                )
                Spacer(modifier = Modifier.height(8.dp))
                Text(
                    text = "You will lose ${formatCentsAsDollars(bonus.bonusAmountCents)} bonus balance.",
                    color = AccentRed,
                    fontSize = 14.sp,
                    fontWeight = FontWeight.SemiBold,
                )
            }
        },
        confirmButton = {
            TextButton(onClick = onConfirm) {
                Text("Forfeit", color = AccentRed, fontWeight = FontWeight.Bold)
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) {
                Text("Keep Bonus", color = AccentGreen)
            }
        },
    )
}
