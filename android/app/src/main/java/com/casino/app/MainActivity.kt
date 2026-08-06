package com.casino.app

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import com.casino.app.ui.navigation.CasinoNavGraph
import com.casino.app.ui.theme.CasinoTheme
import dagger.hilt.android.AndroidEntryPoint

/**
 * Main entry point Activity for the Casino application.
 *
 * Annotated with [AndroidEntryPoint] to enable Hilt dependency injection.
 * Configures edge-to-edge display and sets the Compose content with the
 * [CasinoTheme] and [CasinoNavGraph] as the root composable.
 *
 * @author Casino Team
 * @since 2026-08-03
 */
@AndroidEntryPoint
class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            CasinoTheme {
                CasinoNavGraph()
            }
        }
    }
}
