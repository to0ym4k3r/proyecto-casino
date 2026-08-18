package com.casino.app

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.navigation.NavHostController
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
 * Warm `casino://verify` deep links (app already running) are routed through
 * [onNewIntent] into the auth graph's NavHostController via
 * [NavHostController.handleDeepLink]; the controller is handed up from
 * [CasinoNavGraph] while the unauthenticated auth graph is alive (finding:
 * warm deep links dropped while the app is already running).
 *
 * @author Casino Team
 * @since 2026-08-03
 */
@AndroidEntryPoint
class MainActivity : ComponentActivity() {

    /** The auth graph's NavHostController while it is composed, null otherwise. */
    private var authNavController: NavHostController? = null

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            CasinoTheme {
                CasinoNavGraph(
                    onAuthNavController = { authNavController = it },
                )
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        authNavController?.handleDeepLink(intent)
    }
}
