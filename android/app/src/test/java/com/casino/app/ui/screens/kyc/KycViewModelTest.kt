package com.casino.app.ui.screens.kyc

import com.casino.app.MainDispatcherRule
import com.casino.app.data.api.dto.KycSubmissionDto
import com.casino.app.data.kyc.KycRepository
import io.mockk.Runs
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.just
import io.mockk.mockk
import java.io.File
import java.io.IOException
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
 * RED (6.3): KycViewModel drives the KYC form and document upload.
 *
 * Spec scenarios:
 * - "GIVEN valid fields WHEN submitting KYC THEN status shows pending"
 * - "GIVEN a selected file >5MB WHEN submitting THEN inline error and no upload attempt"
 * - Client-side size/type validation, inline error, no upload >5MB, pending status shown.
 */
@OptIn(ExperimentalCoroutinesApi::class)
class KycViewModelTest {

    @get:Rule
    val mainDispatcherRule = MainDispatcherRule()

    private val repository = mockk<KycRepository>()

    private val pendingSubmission = KycSubmissionDto(
        id = "k-1",
        userId = "u-1",
        fullName = "Alice Wong",
        status = "pending",
        submittedAt = "2026-08-15T10:00:00Z",
    )

    @Test
    fun `form field changes update the state`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.status() } throws HttpException(
                Response.error<Any>(404, """{"success":false,"error":"not found"}""".toResponseBody("application/json".toMediaType()))
            )

            val viewModel = KycViewModel(repository)
            advanceUntilIdle()
            viewModel.onFullNameChange("Alice Wong")
            viewModel.onNationalityChange("ARG")

            assertEquals("Alice Wong", viewModel.uiState.value.fullName)
            assertEquals("ARG", viewModel.uiState.value.nationality)
        }

    @Test
    fun `submit KYC succeeds and shows pending status`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.status() } throws HttpException(
                Response.error<Any>(404, """{"success":false,"error":"not found"}""".toResponseBody("application/json".toMediaType()))
            )
            coEvery { repository.submit(any()) } returns pendingSubmission

            val viewModel = KycViewModel(repository)
            advanceUntilIdle()
            viewModel.onFullNameChange("Alice Wong")
            viewModel.onNationalityChange("ARG")
            viewModel.onDocumentTypeChange("dni")
            viewModel.onDocumentNumberChange("12345678")
            viewModel.submitKyc()
            advanceUntilIdle()

            assertFalse(viewModel.uiState.value.isLoading)
            assertNull(viewModel.uiState.value.submissionError)
            assertTrue(viewModel.uiState.value.submitted)
            assertEquals("pending", viewModel.uiState.value.status)
            coVerify(exactly = 1) { repository.submit(any()) }
        }

    @Test
    fun `submit KYC error shows error message`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.status() } throws HttpException(
                Response.error<Any>(404, """{"success":false,"error":"not found"}""".toResponseBody("application/json".toMediaType()))
            )
            coEvery { repository.submit(any()) } throws HttpException(
                Response.error<Any>(409, """{"success":false,"error":"already submitted"}""".toResponseBody("application/json".toMediaType()))
            )

            val viewModel = KycViewModel(repository)
            advanceUntilIdle()
            viewModel.onFullNameChange("Alice")
            viewModel.submitKyc()
            advanceUntilIdle()

            assertEquals("already submitted", viewModel.uiState.value.submissionError)
        }

    @Test
    fun `upload file larger than 5MB shows inline error without network call`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.status() } throws HttpException(
                Response.error<Any>(404, """{"success":false,"error":"not found"}""".toResponseBody("application/json".toMediaType()))
            )

            val viewModel = KycViewModel(repository)
            advanceUntilIdle()

            // Create a temporary file larger than 5MB
            val tempFile = File.createTempFile("test", ".jpg").apply {
                deleteOnExit()
                outputStream().use { it.write(ByteArray(6 * 1024 * 1024)) }
            }

            viewModel.onDocumentSelected(tempFile)
            viewModel.uploadDocument()
            advanceUntilIdle()

            assertEquals("File must be 5 MB or smaller.", viewModel.uiState.value.documentError)
            coVerify(exactly = 0) { repository.uploadDocument(any()) }
        }

    @Test
    fun `upload disallowed file type shows inline error`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.status() } throws HttpException(
                Response.error<Any>(404, """{"success":false,"error":"not found"}""".toResponseBody("application/json".toMediaType()))
            )

            val viewModel = KycViewModel(repository)
            advanceUntilIdle()

            val tempFile = File.createTempFile("test", ".exe").apply {
                deleteOnExit()
            }

            viewModel.onDocumentSelected(tempFile)
            viewModel.uploadDocument()
            advanceUntilIdle()

            assertEquals("Only JPG, PNG, and PDF files are accepted.", viewModel.uiState.value.documentError)
            coVerify(exactly = 0) { repository.uploadDocument(any()) }
        }

    @Test
    fun `upload success clears document error`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.status() } throws HttpException(
                Response.error<Any>(404, """{"success":false,"error":"not found"}""".toResponseBody("application/json".toMediaType()))
            )
            coEvery { repository.uploadDocument(any()) } returns mockk()

            val viewModel = KycViewModel(repository)
            advanceUntilIdle()

            val tempFile = File.createTempFile("test", ".jpg").apply {
                deleteOnExit()
            }

            viewModel.onDocumentSelected(tempFile)
            viewModel.uploadDocument()
            advanceUntilIdle()

            assertNull(viewModel.uiState.value.documentError)
            assertEquals("Uploaded", viewModel.uiState.value.documentSuccess)
            assertTrue(viewModel.uiState.value.uploadComplete)
        }

    @Test
    fun `double submit is guarded`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.status() } throws HttpException(
                Response.error<Any>(404, """{"success":false,"error":"not found"}""".toResponseBody("application/json".toMediaType()))
            )
            coEvery { repository.submit(any()) } returns pendingSubmission

            val viewModel = KycViewModel(repository)
            advanceUntilIdle()
            viewModel.onFullNameChange("Alice")

            viewModel.submitKyc()
            viewModel.submitKyc()
            advanceUntilIdle()

            coVerify(exactly = 1) { repository.submit(any()) }
        }
}
