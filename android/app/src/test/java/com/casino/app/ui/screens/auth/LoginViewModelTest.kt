package com.casino.app.ui.screens.auth

import com.casino.app.MainDispatcherRule
import com.casino.app.data.api.dto.ApiEnvelope
import com.casino.app.data.api.dto.TokenPair
import com.casino.app.data.auth.AuthRepository
import io.mockk.Runs
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.just
import io.mockk.mockk
import java.io.IOException
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.delay
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.ResponseBody.Companion.toResponseBody
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Rule
import org.junit.Test
import androidx.lifecycle.SavedStateHandle
import retrofit2.HttpException
import retrofit2.Response

/**
 * RED (5.6): LoginViewModel drives the login flow against [AuthRepository].
 *
 * Spec scenario: "GIVEN wrong credentials WHEN submitting login THEN error state
 * shown and no navigation" — the ViewModel has NO navigation API; on success the
 * auth gate reacts to the session flow. Loading state + double-submit guard are
 * required by the design's login/register UI requirement.
 */
@OptIn(ExperimentalCoroutinesApi::class)
class LoginViewModelTest {

    @get:Rule
    val mainDispatcherRule = MainDispatcherRule()

    private val repository = mockk<AuthRepository>()

    @Test
    fun `successful login clears loading and error without navigating`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.login("alice@casino.dev", "secret") } just Runs

            val viewModel = LoginViewModel(repository, SavedStateHandle())
            viewModel.onEmailChange("  alice@casino.dev  ")
            viewModel.onPasswordChange("secret")
            viewModel.login()
            advanceUntilIdle()

            assertEquals(false, viewModel.uiState.value.isLoading)
            assertNull(viewModel.uiState.value.error)
            coVerify(exactly = 1) { repository.login("alice@casino.dev", "secret") }
        }

    @Test
    fun `wrong credentials surface the backend error message inline`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.login(any(), any()) } throws HttpException(
                Response.error<ApiEnvelope<TokenPair>>(
                    401,
                    """{"success":false,"error":"invalid credentials"}"""
                        .toResponseBody("application/json".toMediaType()),
                ),
            )

            val viewModel = LoginViewModel(repository, SavedStateHandle())
            viewModel.onEmailChange("alice@casino.dev")
            viewModel.onPasswordChange("wrong")
            viewModel.login()
            advanceUntilIdle()

            assertEquals("invalid credentials", viewModel.uiState.value.error)
            assertEquals(false, viewModel.uiState.value.isLoading)
        }

    @Test
    fun `network failure surfaces a connection message`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.login(any(), any()) } throws IOException("connection reset")

            val viewModel = LoginViewModel(repository, SavedStateHandle())
            viewModel.onEmailChange("alice@casino.dev")
            viewModel.onPasswordChange("secret")
            viewModel.login()
            advanceUntilIdle()

            assertEquals(
                "Network error. Check your connection and try again.",
                viewModel.uiState.value.error,
            )
        }

    @Test
    fun `blank fields show an inline error without calling the network`() =
        runTest(mainDispatcherRule.testDispatcher) {
            val viewModel = LoginViewModel(repository, SavedStateHandle())
            viewModel.login()

            assertEquals("Enter your email and password.", viewModel.uiState.value.error)
            coVerify(exactly = 0) { repository.login(any(), any()) }
        }

    @Test
    fun `double submit while loading is ignored`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.login(any(), any()) } coAnswers {
                delay(10_000)
                Unit
            }

            val viewModel = LoginViewModel(repository, SavedStateHandle())
            viewModel.onEmailChange("alice@casino.dev")
            viewModel.onPasswordChange("secret")

            viewModel.login()
            viewModel.login()
            runCurrent()

            assertEquals(true, viewModel.uiState.value.isLoading)
            coVerify(exactly = 1) { repository.login("alice@casino.dev", "secret") }

            advanceUntilIdle()
            assertEquals(false, viewModel.uiState.value.isLoading)
            assertNull(viewModel.uiState.value.error)
        }

    @Test
    fun `cancellation is rethrown and never surfaces as a user-facing error`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.login(any(), any()) } throws CancellationException("cancelled")

            val viewModel = LoginViewModel(repository, SavedStateHandle())
            viewModel.onEmailChange("alice@casino.dev")
            viewModel.onPasswordChange("secret")
            viewModel.login()
            advanceUntilIdle()

            assertNull(viewModel.uiState.value.error)
            assertEquals(true, viewModel.uiState.value.isLoading)
        }

    @Test
    fun `verified banner is shown when the verify screen set the flag and is consumed on dismiss`() =
        runTest(mainDispatcherRule.testDispatcher) {
            val savedStateHandle = SavedStateHandle(mapOf("email_verified" to true))

            val viewModel = LoginViewModel(repository, savedStateHandle)
            assertEquals(true, viewModel.verifiedBanner.value)

            viewModel.consumeVerifiedBanner()
            assertEquals(false, viewModel.verifiedBanner.value)
        }
}
