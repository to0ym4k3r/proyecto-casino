package com.casino.app.ui.screens.auth

import androidx.lifecycle.SavedStateHandle
import com.casino.app.MainDispatcherRule
import com.casino.app.data.api.dto.ApiEnvelope
import com.casino.app.data.api.dto.VerifyEmailResponse
import com.casino.app.data.auth.AuthRepository
import io.mockk.Runs
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.just
import io.mockk.mockk
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.runTest
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.ResponseBody.Companion.toResponseBody
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Rule
import org.junit.Test
import retrofit2.HttpException
import retrofit2.Response

/**
 * RED (5.8): VerifyViewModel verifies the email token.
 *
 * Spec scenario: "GIVEN the user opens the verify deep link WHEN the app
 * resolves the link THEN the token is submitted automatically and success
 * navigates to login". The deep link arrives as the `token` route argument in
 * the SavedStateHandle, so verification starts in `init`. A paste fallback is
 * provided for users who open the screen without a deep link.
 */
@OptIn(ExperimentalCoroutinesApi::class)
class VerifyViewModelTest {

    @get:Rule
    val mainDispatcherRule = MainDispatcherRule()

    private val repository = mockk<AuthRepository>()

    @Test
    fun `deep link token is submitted automatically on creation`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.verifyEmail(any()) } just Runs

            val viewModel = VerifyViewModel(
                repository,
                SavedStateHandle(mapOf("token" to "tok-123")),
            )
            advanceUntilIdle()

            assertEquals(true, viewModel.uiState.value.verified)
            assertEquals(false, viewModel.uiState.value.isLoading)
            assertNull(viewModel.uiState.value.error)
            coVerify(exactly = 1) { repository.verifyEmail("tok-123") }
        }

    @Test
    fun `expired token surfaces the backend message inline`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.verifyEmail(any()) } throws HttpException(
                Response.error<ApiEnvelope<VerifyEmailResponse>>(
                    400,
                    """{"success":false,"error":"invalid or expired token"}"""
                        .toResponseBody("application/json".toMediaType()),
                ),
            )

            val viewModel = VerifyViewModel(
                repository,
                SavedStateHandle(mapOf("token" to "tok-dead")),
            )
            advanceUntilIdle()

            assertEquals("invalid or expired token", viewModel.uiState.value.error)
            assertEquals(false, viewModel.uiState.value.verified)
        }

    @Test
    fun `pasted token verifies when the user submits`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.verifyEmail(any()) } just Runs

            val viewModel = VerifyViewModel(repository, SavedStateHandle())
            viewModel.onTokenChange("  tok-pasted  ")
            viewModel.verify()
            advanceUntilIdle()

            assertEquals(true, viewModel.uiState.value.verified)
            coVerify(exactly = 1) { repository.verifyEmail("tok-pasted") }
        }

    @Test
    fun `cancellation is rethrown and never surfaces as a user-facing error`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.verifyEmail(any()) } throws CancellationException("cancelled")

            val viewModel = VerifyViewModel(
                repository,
                SavedStateHandle(mapOf("token" to "tok-123")),
            )
            advanceUntilIdle()

            assertNull(viewModel.uiState.value.error)
            assertEquals(true, viewModel.uiState.value.isLoading)
        }

    @Test
    fun `blank token shows an inline error without calling the network`() =
        runTest(mainDispatcherRule.testDispatcher) {
            val viewModel = VerifyViewModel(repository, SavedStateHandle())
            viewModel.verify()

            assertEquals(
                "Paste the verification token from your email.",
                viewModel.uiState.value.error,
            )
            coVerify(exactly = 0) { repository.verifyEmail(any()) }
        }
}
