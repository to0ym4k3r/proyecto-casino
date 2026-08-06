/**
 * Navigation graph and route definitions for the Casino application.
 *
 * Defines the bottom navigation structure with four primary destinations:
 * Lobby (game listing), Game (active play), Wallet (balance management),
 * and Profile (user settings). Includes the [CasinoNavGraph] composable
 * that wires up Navigation Compose with a bottom navigation bar.
 *
 * @author Casino Team
 * @since 2026-08-03
 */
package com.casino.app.ui.navigation

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
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.navigation.NavDestination.Companion.hierarchy
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import com.casino.app.ui.screens.game.GameScreen
import com.casino.app.ui.screens.lobby.LobbyScreen
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
 * Root navigation composable for the Casino application.
 *
 * Sets up a [Scaffold] with a bottom navigation bar and a [NavHost]
 * that routes between the four primary screens: Lobby, Game, Wallet,
 * and Profile. Handles navigation state, back stack management, and
 * bottom bar icon selection based on the current destination.
 */
@Composable
fun CasinoNavGraph() {
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
            composable(Screen.Lobby.route) { LobbyScreen() }
            composable(Screen.Game.route) { GameScreen() }
            composable(Screen.Wallet.route) { WalletScreen() }
            composable(Screen.Profile.route) { ProfileScreen() }
        }
    }
}
