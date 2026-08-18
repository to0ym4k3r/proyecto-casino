package com.casino.app.ui.screens.wallet

import androidx.compose.foundation.background
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
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material.icons.filled.ContentCopy
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FilterChipDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.casino.app.ui.theme.AccentGreen
import com.casino.app.ui.theme.AccentRed
import com.casino.app.ui.theme.BgDeep
import com.casino.app.ui.theme.BgSurface
import com.casino.app.ui.theme.TextMuted
import com.casino.app.ui.theme.TextPrimary
import com.casino.app.ui.theme.TextSecondary

@Composable
fun DepositScreen(
    uiState: WalletUiState,
    onGetSolDeposit: () -> Unit,
    onBack: () -> Unit,
) {
    val clipboard = LocalClipboardManager.current
    var selectedNetwork by remember { mutableStateOf("SOL") }

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
                text = "Deposit",
                color = TextPrimary,
                fontSize = 20.sp,
                fontWeight = FontWeight.Bold,
            )
        }

        Spacer(modifier = Modifier.height(16.dp))

        // Network selector
        Row(horizontalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(8.dp)) {
            listOf("SOL", "USDC").forEach { network ->
                FilterChip(
                    selected = selectedNetwork == network,
                    onClick = { selectedNetwork = network },
                    label = { Text(network, fontSize = 13.sp, fontWeight = FontWeight.SemiBold) },
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

        Spacer(modifier = Modifier.height(16.dp))

        Button(
            onClick = onGetSolDeposit,
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
                text = if (uiState.isDepositing) "Generating..." else "Get $selectedNetwork Deposit Address",
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

            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .clip(RoundedCornerShape(12.dp))
                    .background(BgSurface)
                    .padding(16.dp),
            ) {
                Column {
                    Text(text = "Your $selectedNetwork Address", color = TextSecondary, fontSize = 13.sp)
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
                    }
                }
            }
        }
    }
}
