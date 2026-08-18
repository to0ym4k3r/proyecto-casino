package com.casino.app.ui.screens.auth

import com.casino.app.MainDispatcherRule
import com.casino.app.data.api.dto.ApiEnvelope
import com.casino.app.data.api.dto.RegisterRequest
import com.casino.app.data.api.dto.TokenPair
import com.casino.app.data.auth.AuthRepository
import io.mockk.Runs
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.just
import io.mockk.mockk
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
 * RED (5.7): RegisterViewModel drives the registration flow.
 *
 * Spec scenario: "GIVEN invalid register data WHEN submitting register THEN
 * inline validation errors shown and no network call" — so validation must
 * happen BEFORE the repository is touched. On success the screen guides to the
 * verify step (email verification), it does not log the user in.
 */
@OptIn(ExperimentalCoroutinesApi::class)
class RegisterViewModelTest {

    @get:Rule
    val mainDispatcherRule = MainDispatcherRule()

    private val repository = mockk<AuthRepository>()

    private fun validForm(vm: RegisterViewModel) {
        vm.onUsernameChange("alice")
        vm.onEmailChange("alice@casino.dev")
        vm.onPasswordChange("s3cret!")
        vm.onFirstNameChange("Alice")
        vm.onLastNameChange("Wonder")
        vm.onDateOfBirthChange("1998-04-12")
    }

    @Test
    fun `valid form registers and reports success`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.register(any()) } just Runs

            val vm = RegisterViewModel(repository, SavedStateHandle())
            validForm(vm)
            vm.submit()
            advanceUntilIdle()

            assertEquals(true, vm.uiState.value.registered)
            assertEquals(false, vm.uiState.value.isLoading)
            assertNull(vm.uiState.value.error)
            coVerify(exactly = 1) {
                repository.register(
                    RegisterRequest(
                        username = "alice",
                        email = "alice@casino.dev",
                        password = "s3cret!",
                        firstName = "Alice",
                        lastName = "Wonder",
                        dateOfBirth = "1998-04-12",
                    ),
                )
            }
        }

    @Test
    fun `malformed email shows inline error without network call`() =
        runTest(mainDispatcherRule.testDispatcher) {
            val vm = RegisterViewModel(repository, SavedStateHandle())
            validForm(vm)
            vm.onEmailChange("not-an-email")
            vm.submit()
            advanceUntilIdle()

            assertEquals("Enter a valid email address.", vm.uiState.value.fieldErrors.email)
            assertEquals(false, vm.uiState.value.registered)
            coVerify(exactly = 0) { repository.register(any()) }
        }

    @Test
    fun `underage date of birth shows inline error without network call`() =
        runTest(mainDispatcherRule.testDispatcher) {
            val vm = RegisterViewModel(repository, SavedStateHandle())
            validForm(vm)
            // 2020-01-01 is always ~6 years old — no time-sensitive boundary
            vm.onDateOfBirthChange("2020-01-01")
            vm.submit()
            advanceUntilIdle()

            assertEquals(
                "You must be at least 18 years old.",
                vm.uiState.value.fieldErrors.dateOfBirth,
            )
            coVerify(exactly = 0) { repository.register(any()) }
        }

    @Test
    fun `conflict surfaces the backend message and does not report success`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.register(any()) } throws HttpException(
                Response.error<ApiEnvelope<TokenPair>>(
                    409,
                    """{"success":false,"error":"username already taken"}"""
                        .toResponseBody("application/json".toMediaType()),
                ),
            )

            val vm = RegisterViewModel(repository, SavedStateHandle())
            validForm(vm)
            vm.submit()
            advanceUntilIdle()

            assertEquals("username already taken", vm.uiState.value.error)
            assertEquals(false, vm.uiState.value.registered)
        }

    @Test
    fun `cancellation is rethrown and never surfaces as a user-facing error`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.register(any()) } throws CancellationException("cancelled")

            val vm = RegisterViewModel(repository, SavedStateHandle())
            validForm(vm)
            vm.submit()
            advanceUntilIdle()

            assertNull(vm.uiState.value.error)
            assertEquals(true, vm.uiState.value.isLoading)
        }

    @Test
    fun `double submit while loading is ignored`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.register(any()) } coAnswers {
                delay(10_000)
            }

            val vm = RegisterViewModel(repository, SavedStateHandle())
            validForm(vm)
            vm.submit()
            vm.submit()
            runCurrent()

            assertEquals(true, vm.uiState.value.isLoading)
            coVerify(exactly = 1) { repository.register(any()) }

            advanceUntilIdle()
            assertEquals(true, vm.uiState.value.registered)
            assertEquals(false, vm.uiState.value.isLoading)
        }
}
