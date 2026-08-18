package com.casino.app.ui.screens.game

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
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
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import com.casino.app.ui.theme.AccentGreen
import com.casino.app.ui.theme.AccentGreenDark
import com.casino.app.ui.theme.AccentRed
import com.casino.app.ui.theme.BgDeep
import com.casino.app.ui.theme.BgElevated
import com.casino.app.ui.theme.BgPrimary
import com.casino.app.ui.theme.BgSurface
import com.casino.app.ui.theme.TextMuted
import com.casino.app.ui.theme.TextPrimary
import com.casino.app.ui.theme.TextSecondary

/**
 * Game screen composable.
 *
 * Displays the active game view with session info, bet input, and
 * bet result feedback. Uses [GameViewModel] via Hilt.
 */
@Composable
fun GameScreen(
    viewModel: GameViewModel = hiltViewModel()
) {
    val uiState by viewModel.uiState.collectAsState()
    var betAmount by remember { mutableStateOf("") }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(BgDeep)
            .padding(16.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        // Loading state
        if (uiState.isLoading) {
            Box(
                modifier = Modifier.fillMaxSize(),
                contentAlignment = Alignment.Center,
            ) {
                CircularProgressIndicator(
                    color = AccentGreen,
                    modifier = Modifier.size(48.dp),
                )
            }
            return
        }

        // Error state
        if (uiState.error != null) {
            Box(
                modifier = Modifier.fillMaxSize(),
                contentAlignment = Alignment.Center,
            ) {
                Text(
                    text = uiState.error ?: "",
                    color = AccentRed,
                    fontSize = 16.sp,
                )
            }
            return
        }

        val game = uiState.game ?: return

        // Game header
        Column(
            modifier = Modifier.fillMaxWidth(),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Text(
                text = game.name,
                color = TextPrimary,
                fontSize = 24.sp,
                fontWeight = FontWeight.Bold,
            )
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text = game.category,
                color = TextMuted,
                fontSize = 14.sp,
            )
        }

        Spacer(modifier = Modifier.height(24.dp))

        // Game info card
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .clip(RoundedCornerShape(12.dp))
                .background(BgPrimary)
                .padding(16.dp),
        ) {
            Column {
                Text(
                    text = "Session Info",
                    color = AccentGreen,
                    fontSize = 16.sp,
                    fontWeight = FontWeight.SemiBold,
                )
                Spacer(modifier = Modifier.height(8.dp))
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                ) {
                    Text("RTP", color = TextSecondary, fontSize = 14.sp)
                    Text("${game.rtp}%", color = TextPrimary, fontSize = 14.sp)
                }
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                ) {
                    Text("Min Bet", color = TextSecondary, fontSize = 14.sp)
                    Text("${game.minBet}¢", color = TextPrimary, fontSize = 14.sp)
                }
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                ) {
                    Text("Max Bet", color = TextSecondary, fontSize = 14.sp)
                    Text("${game.maxBet}¢", color = TextPrimary, fontSize = 14.sp)
                }
                if (uiState.serverSeedHash != null) {
                    Spacer(modifier = Modifier.height(8.dp))
                    Text(
                        text = "Server Seed Hash",
                        color = TextMuted,
                        fontSize = 12.sp,
                    )
                    Text(
                        text = uiState.serverSeedHash?.take(16) ?: "",
                        color = TextSecondary,
                        fontSize = 12.sp,
                    )
                }
            }
        }

        Spacer(modifier = Modifier.height(24.dp))

        // Bet input
        OutlinedTextField(
            value = betAmount,
            onValueChange = { betAmount = it.filter { c -> c.isDigit() } },
            modifier = Modifier.fillMaxWidth(),
            label = { Text("Bet Amount (¢)", color = TextMuted) },
            placeholder = { Text("Enter amount", color = TextDimmed) },
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
            shape = RoundedCornerShape(8.dp),
            colors = OutlinedTextFieldDefaults.colors(
                focusedBorderColor = AccentGreen,
                unfocusedBorderColor = BgSurface,
                focusedContainerColor = BgPrimary,
                unfocusedContainerColor = BgPrimary,
                cursorColor = AccentGreen,
                focusedTextColor = TextPrimary,
                unfocusedTextColor = TextPrimary,
            ),
            singleLine = true,
            enabled = !uiState.isPlacingBet,
        )

        Spacer(modifier = Modifier.height(16.dp))

        // Place bet button
        Button(
            onClick = {
                val amount = betAmount.toLongOrNull() ?: return@Button
                viewModel.placeBet(amount)
                betAmount = ""
            },
            modifier = Modifier
                .fillMaxWidth()
                .height(56.dp),
            shape = RoundedCornerShape(8.dp),
            colors = ButtonDefaults.buttonColors(
                containerColor = AccentGreen,
                contentColor = BgDeep,
                disabledContainerColor = AccentGreenDark,
            ),
            enabled = !uiState.isPlacingBet && betAmount.isNotBlank(),
        ) {
            if (uiState.isPlacingBet) {
                CircularProgressIndicator(
                    color = BgDeep,
                    modifier = Modifier.size(24.dp),
                    strokeWidth = 2.dp,
                )
            } else {
                Text(
                    text = "PLACE BET",
                    fontSize = 16.sp,
                    fontWeight = FontWeight.Bold,
                )
            }
        }

        // Bet error
        AnimatedVisibility(visible = uiState.betError != null) {
            Text(
                text = uiState.betError ?: "",
                color = AccentRed,
                fontSize = 14.sp,
                modifier = Modifier.padding(top = 8.dp),
            )
        }

        // Bet result
        AnimatedVisibility(
            visible = uiState.lastBet != null,
            enter = fadeIn(),
            exit = fadeOut(),
        ) {
            val bet = uiState.lastBet ?: return@AnimatedVisibility
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(top = 16.dp)
                    .clip(RoundedCornerShape(12.dp))
                    .background(if (bet.won) AccentGreen.copy(alpha = 0.15f) else AccentRed.copy(alpha = 0.15f))
                    .padding(16.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Text(
                    text = if (bet.won) "YOU WON!" else "YOU LOST",
                    color = if (bet.won) AccentGreen else AccentRed,
                    fontSize = 20.sp,
                    fontWeight = FontWeight.Bold,
                )
                Spacer(modifier = Modifier.height(8.dp))
                Text(
                    text = "Bet: ${bet.amount}¢  |  Payout: ${bet.payout}¢",
                    color = TextSecondary,
                    fontSize = 14.sp,
                )
                Text(
                    text = "Multiplier: ${String.format("%.2f", bet.multiplier)}x",
                    color = TextMuted,
                    fontSize = 12.sp,
                )
            }
        }

        Spacer(modifier = Modifier.weight(1f))

        // End session button
        Button(
            onClick = { viewModel.endSession() },
            modifier = Modifier
                .fillMaxWidth()
                .height(48.dp),
            shape = RoundedCornerShape(8.dp),
            colors = ButtonDefaults.buttonColors(
                containerColor = BgSurface,
                contentColor = TextSecondary,
            ),
        ) {
            Text(
                text = "END SESSION",
                fontSize = 14.sp,
                fontWeight = FontWeight.SemiBold,
            )
        }
    }
}

private val TextDimmed = com.casino.app.ui.theme.TextDimmed
