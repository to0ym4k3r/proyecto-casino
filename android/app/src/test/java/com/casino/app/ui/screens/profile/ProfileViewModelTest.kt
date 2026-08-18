package com.casino.app.ui.screens.profile

import com.casino.app.MainDispatcherRule
import com.casino.app.data.api.dto.UpdateProfileRequest
import com.casino.app.data.api.dto.UserDto
import com.casino.app.data.profile.ProfileRepository
import io.mockk.Runs
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.just
import io.mockk.mockk
import java.io.IOException
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.runTest
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.ResponseBody.Companion.toResponseBody
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import retrofit2.HttpException
import retrofit2.Response

/**
 * RED (6.2): ProfileViewModel loads profile on init and saves via PUT.
 *
 * Spec scenario: "GIVEN an authenticated session WHEN opening Profile THEN
 * current profile renders" — ViewModel loads on construction. Saving calls PUT
 * and shows updated data (loading/error/success states).
 */
@OptIn(ExperimentalCoroutinesApi::class)
class ProfileViewModelTest {

    @get:Rule
    val mainDispatcherRule = MainDispatcherRule()

    private val repository = mockk<ProfileRepository>()

    private val testUser = UserDto(
        id = "u-1",
        username = "alice",
        email = "alice@casino.dev",
        firstName = "Alice",
        lastName = "Wong",
        dateOfBirth = "1998-03-12T00:00:00Z",
        isActive = true,
        createdAt = "2026-08-01T10:00:00Z",
        updatedAt = "2026-08-01T10:00:00Z",
    )

    private val updatedUser = testUser.copy(username = "neo_alice", updatedAt = "2026-08-05T12:00:00Z")

    @Test
    fun `initial load sets the profile on success`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getProfile() } returns testUser

            val viewModel = ProfileViewModel(repository)
            advanceUntilIdle()

            assertFalse(viewModel.uiState.value.isLoading)
            assertNull(viewModel.uiState.value.error)
            assertEquals("alice", viewModel.uiState.value.username)
            assertEquals("Alice", viewModel.uiState.value.firstName)
            assertEquals("alice@casino.dev", viewModel.uiState.value.email)
        }

    @Test
    fun `load error sets error state`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getProfile() } throws IOException("connection reset")

            val viewModel = ProfileViewModel(repository)
            advanceUntilIdle()

            assertFalse(viewModel.uiState.value.isLoading)
            assertEquals("Network error. Check your connection and try again.", viewModel.uiState.value.error)
        }

    @Test
    fun `save updates profile on success`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getProfile() } returns testUser
            coEvery { repository.updateProfile(any()) } returns updatedUser

            val viewModel = ProfileViewModel(repository)
            advanceUntilIdle()
            viewModel.onUsernameChange("neo_alice")
            viewModel.save()
            advanceUntilIdle()

            assertFalse(viewModel.uiState.value.isLoading)
            assertNull(viewModel.uiState.value.error)
            assertEquals("neo_alice", viewModel.uiState.value.username)
            coVerify(exactly = 1) { repository.updateProfile(
                UpdateProfileRequest(username = "neo_alice", firstName = "Alice", lastName = "Wong")
            ) }
        }

    @Test
    fun `save error sets error state`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getProfile() } returns testUser
            coEvery { repository.updateProfile(any()) } throws HttpException(
                Response.error<Any>(400, """{"success":false,"error":"username taken"}""".toResponseBody("application/json".toMediaType()))
            )

            val viewModel = ProfileViewModel(repository)
            advanceUntilIdle()
            viewModel.onUsernameChange("taken_name")
            viewModel.save()
            advanceUntilIdle()

            assertFalse(viewModel.uiState.value.isLoading)
            assertEquals("username taken", viewModel.uiState.value.error)
        }
}
