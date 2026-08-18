package com.casino.app.ui.screens.wallet

import androidx.compose.foundation.background
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
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.Alignment
import com.casino.app.ui.theme.AccentGreen
import com.casino.app.ui.theme.AccentRed
import com.casino.app.ui.theme.BgDeep
import com.casino.app.ui.theme.BgSurface
import com.casino.app.ui.theme.TextPrimary
import com.casino.app.ui.theme.TextSecondary

@Composable
fun WithdrawScreen(
    uiState: WalletUiState,
    onAmountChange: (String) -> Unit,
    onAddressChange: (String) -> Unit,
    onWithdraw: () -> Unit,
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
                text = "Withdraw SOL",
                color = TextPrimary,
                fontSize = 20.sp,
                fontWeight = FontWeight.Bold,
            )
        }

        Spacer(modifier = Modifier.height(24.dp))

        OutlinedTextField(
            value = uiState.withdrawAmountInput,
            onValueChange = onAmountChange,
            label = { Text("Amount (USD)", color = TextSecondary) },
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

        OutlinedTextField(
            value = uiState.withdrawAddress,
            onValueChange = onAddressChange,
            label = { Text("Recipient SOL Address", color = TextSecondary) },
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
            onClick = onWithdraw,
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
