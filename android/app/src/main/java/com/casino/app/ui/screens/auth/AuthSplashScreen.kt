package com.casino.app.ui.screens.auth

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.casino.app.ui.theme.AccentGreen
import com.casino.app.ui.theme.BgDeep
import com.casino.app.ui.theme.TextSecondary

/**
 * Transient branding shown while the auth gate waits for the FIRST real
 * session emission (deviation #12: the store-backed flow has no synthetic
 * initial value, so a pre-existing session never flashes the login screen).
 */
@Composable
fun AuthSplashScreen() {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(BgDeep),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center,
    ) {
        Text(
            text = "CASINO",
            color = AccentGreen,
            fontSize = 36.sp,
            fontWeight = FontWeight.ExtraBold,
        )
        Text(
            text = "Loading…",
            color = TextSecondary,
            fontSize = 13.sp,
            modifier = Modifier.padding(top = 8.dp),
        )
    }
}
