package com.casino.app.ui.navigation

import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.navigation.NavBackStackEntry
import androidx.navigation.NavController
import androidx.navigation.NavHostController
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.navigation.navArgument
import androidx.navigation.navDeepLink
import com.casino.app.ui.screens.auth.LoginScreen
import com.casino.app.ui.screens.auth.RegisterScreen
import com.casino.app.ui.screens.auth.VerifyScreen
import android.net.Uri

/** Routes of the unauthenticated auth graph (task 5.6–5.8). */
object AuthRoute {
    const val Login = "login"
    const val Register = "register"
    const val VerifyPattern = "verify?token={token}"

    /** Concrete route for [VerifyPattern] with [token] URL-encoded. */
    fun verify(token: String): String = "verify?token=${Uri.encode(token)}"
}

/**
 * Returns the login back-stack entry when present, or null when login is not
 * on the back stack. Navigation 2.7.7 [NavController.getBackStackEntry] throws
 * [IllegalArgumentException] for a missing route — the crash path when a
 * cold-start `casino://verify` deep link leaves `verify` as the ONLY back
 * stack entry (finding: cold-start deep-link crash).
 */
internal fun NavController.loginBackStackEntryOrNull(): NavBackStackEntry? =
    try {
        getBackStackEntry(AuthRoute.Login)
    } catch (ignored: IllegalArgumentException) {
        null
    }

/**
 * Navigation graph for the unauthenticated area: login → register → verify
 * (task 5.6–5.8).
 *
 * On verification success the graph hands an "email verified" flag to the
 * login back-stack entry and pops back to login, which shows the banner
 * (the user signs in with the now-verified account).
 *
 * @param initialToken When the app was cold-started from the
 *   `casino://verify?token={token}` deep link, the graph starts directly at
 *   the verify destination so the token submits automatically. Null starts at
 *   login as usual. The nested NavHost cannot resolve an external deep link on
 *   its own (the root gate is not a NavHost), so the root forwards the token;
 *   while this graph is alive, [androidx.navigation.navDeepLink] additionally
 *   handles warm `casino://verify` links.
 * @param onAuthNavController Exposes this graph's NavHostController to the
 *   caller (the Activity) so warm `casino://verify` deep links are routed
 *   through [NavHostController.handleDeepLink] (finding: warm deep links
 *   dropped while the app is already running).
 */
@Composable
fun AuthNavGraph(
    initialToken: String?,
    onAuthNavController: (NavHostController?) -> Unit = {},
) {
    val navController = rememberNavController()

    // Hand the controller up for warm deep-link handling and clear it when the
    // graph leaves composition (its NavHost is no longer active).
    DisposableEffect(navController) {
        onAuthNavController(navController)
        onDispose { onAuthNavController(null) }
    }

    val startDestination = if (initialToken != null) {
        AuthRoute.verify(initialToken)
    } else {
        AuthRoute.Login
    }

    NavHost(
        navController = navController,
        startDestination = startDestination,
        modifier = Modifier,
    ) {
        composable(AuthRoute.Login) {
            LoginScreen(
                onNavigateToRegister = { navController.navigate(AuthRoute.Register) },
            )
        }
        composable(AuthRoute.Register) {
            RegisterScreen(
                onNavigateToVerify = { navController.navigate(AuthRoute.verify("")) },
                onNavigateToLogin = {
                    navController.popBackStack(AuthRoute.Login, inclusive = false)
                },
            )
        }
        composable(
            route = AuthRoute.VerifyPattern,
            arguments = listOf(
                navArgument("token") {
                    type = NavType.StringType
                    defaultValue = ""
                },
            ),
            deepLinks = listOf(
                navDeepLink { uriPattern = "casino://verify?token={token}" },
            ),
        ) {
            VerifyScreen(
                onNavigateToLogin = {
                    val loginEntry = navController.loginBackStackEntryOrNull()
                    if (loginEntry != null) {
                        // Normal flow: hand the "email verified" flag to login
                        // and pop back to it.
                        loginEntry.savedStateHandle["email_verified"] = true
                        navController.popBackStack(AuthRoute.Login, inclusive = false)
                    } else {
                        // Cold-start deep link: [verify] is the only back-stack
                        // entry — start a clean login instead of crashing on
                        // the missing lookup (finding: cold-start crash).
                        navController.navigate(AuthRoute.Login) {
                            popUpTo(0) { inclusive = true }
                        }
                    }
                },
            )
        }
    }
}
