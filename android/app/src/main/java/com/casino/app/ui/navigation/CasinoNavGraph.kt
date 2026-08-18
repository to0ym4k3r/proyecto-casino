/**
 * Navigation graph and route definitions for the Casino application.
 *
 * The root [CasinoNavGraph] is the SESSION GATE (task 5.5): it observes
 * [AuthGateViewModel.destination] and renders the splash, the authenticated
 * main tabs ([MainNavGraph]) or the unauthenticated auth flow
 * ([AuthNavGraph]). Two independent NavHosts mean a session flip (login or
 * logout) naturally clears the other graph's back stack.
 *
 * [Screen] defines the bottom navigation structure with four primary
 * destinations: Lobby (game listing), Game (active play), Wallet (balance
 * management), and Profile (user settings).
 *
 * @author Casino Team
 * @since 2026-08-03
 */
package com.casino.app.ui.navigation

import android.app.Activity
import android.content.Context
import android.content.ContextWrapper
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Casino
import androidx.compose.material.icons.filled.Gamepad
import androidx.compose.material.icons.filled.Person
import androidx.compose.material.icons.filled.Wallet
import androidx.compose.material.icons.outlined.Casino
import androidx.compose.material.icons.outlined.Gamepad
import androidx.compose.material.icons.outlined.Person
import androidx.compose.material.icons.outlined.Wallet
import androidx.compose.material3.Icon
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.NavigationBarItemDefaults
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavDestination.Companion.hierarchy
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import com.casino.app.ui.screens.auth.AuthSplashScreen
import com.casino.app.ui.screens.game.GameScreen
import com.casino.app.ui.screens.lobby.LobbyScreen
import com.casino.app.ui.screens.kyc.KycScreen
import com.casino.app.ui.screens.profile.ProfileScreen
import com.casino.app.ui.screens.wallet.WalletScreen
import com.casino.app.ui.theme.AccentGreen
import com.casino.app.ui.theme.BgDeep
import com.casino.app.ui.theme.TextMuted

/**
 * Sealed class representing a navigation destination in the Casino app.
 *
 * Each destination defines its route, display label, and icons for
 * selected/unselected states in the bottom navigation bar.
 *
 * @property route The navigation route string used by Navigation Compose.
 * @property label The user-facing display label for the navigation item.
 * @property selectedIcon The icon shown when this destination is active.
 * @property unselectedIcon The icon shown when this destination is inactive.
 */
sealed class Screen(
    val route: String,
    val label: String,
    val selectedIcon: ImageVector,
    val unselectedIcon: ImageVector
) {
    data object Lobby : Screen("lobby", "Games", Icons.Filled.Gamepad, Icons.Outlined.Gamepad)
    data object Game : Screen("game", "Play", Icons.Filled.Casino, Icons.Outlined.Casino)
    data object Wallet : Screen("wallet", "Wallet", Icons.Filled.Wallet, Icons.Outlined.Wallet)
    data object Profile : Screen("profile", "Profile", Icons.Filled.Person, Icons.Outlined.Person)
}

/** The list of all bottom navigation destinations displayed in the bottom bar. */
private val bottomNavItems = listOf(
    Screen.Lobby,
    Screen.Game,
    Screen.Wallet,
    Screen.Profile
)

/**
 * Extracts the `casino://verify?token=...` cold-start token from a deep link,
 * or null when [scheme]/[host] are not the verify link or the token is absent.
 * The uri pieces are pre-split so the decision stays unit-testable without the
 * Android Uri runtime.
 */
internal fun verifyTokenFrom(scheme: String?, host: String?, token: String?): String? =
    if (scheme == "casino" && host == "verify") token else null

/**
 * Root composable: the session gate (task 5.5).
 *
 * Renders the splash while the gate resolves the session, then the main tabs
 * for an authenticated session or the auth flow otherwise. The cold-start
 * `casino://verify?token={token}` deep link token is extracted from the
 * activity intent here and forwarded to [AuthNavGraph], whose nested NavHost
 * cannot resolve external deep links on its own (known limitation: a verify
 * link opened while the user is already signed in is not applied).
 *
 * @param onAuthNavController Exposes the auth graph's NavHostController to the
 *   caller (the Activity) so warm `casino://verify` deep links are routed via
 *   [NavHostController.handleDeepLink] (finding: warm deep links dropped).
 */
@Composable
fun CasinoNavGraph(
    viewModel: AuthGateViewModel = hiltViewModel(),
    onAuthNavController: (NavHostController?) -> Unit = {},
) {
    val destination by viewModel.destination.collectAsStateWithLifecycle()

    val context = LocalContext.current
    var initialVerifyToken by remember {
        val data = context.findActivity()?.intent?.data
        mutableStateOf(verifyTokenFrom(data?.scheme, data?.host, data?.getQueryParameter("token")))
    }

    // Consume the cold-start verify token once the user reaches the signed-in
    // area: the auth graph is recreated after a logout, and a remembered token
    // would land the user on an already-consumed verify screen (finding: stale
    // remembered verify token after logout). Consuming on the Main transition
    // (instead of right after first composition) keeps the cold-start verify
    // graph alive on its token for its whole visit.
    LaunchedEffect(destination) {
        if (destination == AuthGateDestination.Main) {
            initialVerifyToken = null
        }
    }

    when (destination) {
        AuthGateDestination.Splash -> AuthSplashScreen()
        AuthGateDestination.Main -> MainNavGraph()
        AuthGateDestination.Auth -> AuthNavGraph(
            initialToken = initialVerifyToken,
            onAuthNavController = onAuthNavController,
        )
    }
}

/**
 * The authenticated area: a [Scaffold] with the bottom navigation bar and a
 * [NavHost] routing between Lobby, Game, Wallet and Profile.
 */
@Composable
private fun MainNavGraph() {
    val navController = rememberNavController()
    val navBackStackEntry by navController.currentBackStackEntryAsState()
    val currentDestination = navBackStackEntry?.destination

    Scaffold(
        containerColor = BgDeep,
        bottomBar = {
            NavigationBar(containerColor = BgDeep) {
                bottomNavItems.forEach { screen ->
                    val selected = currentDestination?.hierarchy?.any { it.route == screen.route } == true
                    NavigationBarItem(
                        selected = selected,
                        onClick = {
                            navController.navigate(screen.route) {
                                popUpTo(navController.graph.findStartDestination().id) {
                                    saveState = true
                                }
                                launchSingleTop = true
                                restoreState = true
                            }
                        },
                        icon = {
                            Icon(
                                imageVector = if (selected) screen.selectedIcon else screen.unselectedIcon,
                                contentDescription = screen.label
                            )
                        },
                        label = {
                            Text(text = screen.label)
                        },
                        colors = NavigationBarItemDefaults.colors(
                            selectedIconColor = AccentGreen,
                            selectedTextColor = AccentGreen,
                            unselectedIconColor = TextMuted,
                            unselectedTextColor = TextMuted,
                            indicatorColor = BgDeep
                        )
                    )
                }
            }
        }
    ) { innerPadding ->
        NavHost(
            navController = navController,
            startDestination = Screen.Lobby.route,
            modifier = Modifier.padding(innerPadding)
        ) {
            composable(Screen.Lobby.route) {
                LobbyScreen(
                    onGameClick = { gameId -> navController.navigate("game/$gameId") },
                )
            }
            composable(Screen.Game.route) { GameScreen() }
            composable("game/{gameId}") { GameScreen() }
            composable(Screen.Wallet.route) { WalletScreen() }
            composable(Screen.Profile.route) {
                ProfileScreen(
                    onNavigateToKyc = { navController.navigate("kyc") },
                )
            }
            composable("kyc") {
                KycScreen(
                    onBack = { navController.popBackStack() },
                )
            }
        }
    }
}

/** Unwraps the nearest [Activity] from any context chain. */
private tailrec fun Context.findActivity(): Activity? = when (this) {
    is Activity -> this
    is ContextWrapper -> baseContext.findActivity()
    else -> null
}
