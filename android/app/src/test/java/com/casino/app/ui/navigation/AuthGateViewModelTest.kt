package com.casino.app.ui.navigation

import androidx.navigation.NavBackStackEntry
import androidx.navigation.NavController
import app.cash.turbine.test
import com.casino.app.MainDispatcherRule
import com.casino.app.data.auth.AuthRepository
import io.mockk.every
import io.mockk.mockk
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Rule
import org.junit.Test

/**
 * RED (5.5): the navigation gate must OBSERVE the auth flow (deviation #12) and
 * switch start destinations from it — never read `.value` once.
 *
 * [AuthGateViewModel] derives the gate destination from [AuthRepository.session]:
 * null/not-yet-resolved → Splash (hold the app root decision), authenticated →
 * Main tabs, unauthenticated → auth flow (login start destination).
 */
@OptIn(ExperimentalCoroutinesApi::class)
class AuthGateViewModelTest {

    @get:Rule
    val mainDispatcherRule = MainDispatcherRule()

    @Test
    fun `destination starts at Splash and resolves to Main when a session exists`() =
        runTest(mainDispatcherRule.testDispatcher) {
            val repository = mockk<AuthRepository>()
            every { repository.session } returns MutableStateFlow(true)

            AuthGateViewModel(repository).destination.test {
                assertEquals(AuthGateDestination.Splash, awaitItem())
                advanceUntilIdle()
                assertEquals(AuthGateDestination.Main, awaitItem())
            }
        }

    @Test
    fun `destination resolves to Auth when no session exists`() =
        runTest(mainDispatcherRule.testDispatcher) {
            val repository = mockk<AuthRepository>()
            every { repository.session } returns MutableStateFlow(false)

            AuthGateViewModel(repository).destination.test {
                assertEquals(AuthGateDestination.Splash, awaitItem())
                advanceUntilIdle()
                assertEquals(AuthGateDestination.Auth, awaitItem())
            }
        }

    @Test
    fun `destination follows session changes (logout returns to Auth)`() =
        runTest(mainDispatcherRule.testDispatcher) {
            val session = MutableStateFlow(true)
            val repository = mockk<AuthRepository>()
            every { repository.session } returns session

            val viewModel = AuthGateViewModel(repository)
            viewModel.destination.test {
                assertEquals(AuthGateDestination.Splash, awaitItem())
                advanceUntilIdle()
                assertEquals(AuthGateDestination.Main, awaitItem())

                session.value = false
                advanceUntilIdle()
                assertEquals(AuthGateDestination.Auth, awaitItem())
            }
        }
}

/**
 * RED (5.5): the pure mapping from raw session state to the gate destination —
 * the single decision point the NavGraph renders against.
 */
class AuthGateDestinationTest {

    @Test
    fun `null session maps to Splash - gate still resolving`() {
        assertEquals(AuthGateDestination.Splash, AuthGateDestination.fromSession(null))
    }

    @Test
    fun `authenticated session maps to Main tabs`() {
        assertEquals(AuthGateDestination.Main, AuthGateDestination.fromSession(true))
    }

    @Test
    fun `unauthenticated session maps to auth flow`() {
        assertEquals(AuthGateDestination.Auth, AuthGateDestination.fromSession(false))
    }
}

/**
 * RED (finding 1 — cold-start deep-link crash): the verify screen must reach
 * login without crashing when login is NOT on the back stack. A cold-start
 * `casino://verify?token={token}` deep link leaves `verify` as the ONLY back
 * stack entry, and Navigation 2.7.7 [NavController.getBackStackEntry] throws
 * [IllegalArgumentException] for a missing route — the banner-handoff lookup
 * must degrade to a null entry instead.
 */
class LoginBackStackEntryTest {

    private val navController = mockk<NavController>()

    @Test
    fun `returns the login entry when login is on the back stack`() {
        val loginEntry = mockk<NavBackStackEntry>()
        every { navController.getBackStackEntry(AuthRoute.Login) } returns loginEntry

        assertEquals(loginEntry, navController.loginBackStackEntryOrNull())
    }

    @Test
    fun `returns null instead of throwing when login is missing from the back stack`() {
        every { navController.getBackStackEntry(AuthRoute.Login) } throws
            IllegalArgumentException("Destination with route login cannot be found")

        assertNull(navController.loginBackStackEntryOrNull())
    }

    @Test
    fun `does not swallow unrelated exceptions`() {
        val boom = IllegalStateException("boom")
        every { navController.getBackStackEntry(AuthRoute.Login) } throws boom

        val thrown = assertThrows(IllegalStateException::class.java) {
            navController.loginBackStackEntryOrNull()
        }
        assertEquals(boom, thrown)
    }
}

/**
 * RED (finding 2 — stale remembered verify token after logout): the cold-start
 * deep-link token must only be extracted from the `casino://verify` link, and
 * the extracted value must be consumable so a recreated auth graph (after
 * logout) starts at login instead of an already-consumed verify screen.
 */
class VerifyTokenExtractionTest {

    @Test
    fun `returns the token for a casino verify deep link`() {
        assertEquals("tok-abc", verifyTokenFrom("casino", "verify", "tok-abc"))
    }

    @Test
    fun `returns null when the scheme is not casino`() {
        assertNull(verifyTokenFrom("https", "verify", "tok-abc"))
    }

    @Test
    fun `returns null when the host is not verify`() {
        assertNull(verifyTokenFrom("casino", "lobby", "tok-abc"))
    }

    @Test
    fun `returns null when there is no deep-link data`() {
        assertNull(verifyTokenFrom(null, null, null))
    }
}
