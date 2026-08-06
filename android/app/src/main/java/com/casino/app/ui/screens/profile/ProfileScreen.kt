/**
 * Profile screen implementation for the Casino application.
 *
 * Displays the user profile view. Currently serves as a placeholder
 * that shows a centered label indicating the profile screen.
 *
 * @author Casino Team
 * @since 2026-08-03
 */
package com.casino.app.ui.screens.profile

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp
import com.casino.app.ui.theme.BgDeep
import com.casino.app.ui.theme.TextMuted

/**
 * Composable that renders the profile screen.
 *
 * Displays a centered "Profile Screen" placeholder text on the deep
 * background color. This is a stub implementation to be replaced
 * with actual user profile content.
 */
@Composable
fun ProfileScreen() {
    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(BgDeep),
        contentAlignment = Alignment.Center
    ) {
        Text(
            text = "Profile Screen",
            color = TextMuted,
            fontSize = 20.sp,
            fontWeight = FontWeight.SemiBold
        )
    }
}
