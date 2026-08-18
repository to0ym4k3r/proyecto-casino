package com.casino.app.ui.screens.kyc

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.casino.app.data.api.dto.KycSubmissionDto
import com.casino.app.data.kyc.KycRepository
import com.casino.app.ui.screens.auth.AuthErrorMessages
import dagger.hilt.android.lifecycle.HiltViewModel
import java.io.File
import javax.inject.Inject
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import okhttp3.MediaType.Companion.toMediaTypeOrNull
import okhttp3.MultipartBody
import okhttp3.RequestBody.Companion.asRequestBody

/** UI state of the KYC screen (form fields, loading, errors, status). */
data class KycUiState(
    // Submission fields
    val fullName: String = "",
    val nationality: String = "",
    val documentType: String = "",
    val documentNumber: String = "",
    // Submission state
    val isLoading: Boolean = false,
    val submissionError: String? = null,
    val submitted: Boolean = false,
    // Status (after submission or on load)
    val status: String? = null,
    val submittedAt: String? = null,
    // Document upload
    val selectedFile: File? = null,
    val isUploading: Boolean = false,
    val documentError: String? = null,
    val documentSuccess: String? = null,
    val uploadComplete: Boolean = false,
)

/**
 * ViewModel for the KYC screen (task 6.3).
 *
 * On construction, checks if a KYC submission already exists (GET /kyc/status).
 * If 404, the user has no submission and can fill the form. If 200, the status
 * is displayed (pending/approved/rejected).
 *
 * Document uploads have client-side validation (≤5 MB, jpg/png/pdf only)
 * before the network call is made. The server also enforces these limits
 * (design D2: 413 oversized, 415 wrong type), but the client-side check
 * prevents unnecessary network traffic.
 */
@HiltViewModel
class KycViewModel @Inject constructor(
    private val kycRepository: KycRepository,
) : ViewModel() {

    private val _uiState = MutableStateFlow(KycUiState())
    val uiState: StateFlow<KycUiState> = _uiState.asStateFlow()

    init {
        checkStatus()
    }

    fun onFullNameChange(value: String) {
        _uiState.update { it.copy(fullName = value) }
    }

    fun onNationalityChange(value: String) {
        _uiState.update { it.copy(nationality = value) }
    }

    fun onDocumentTypeChange(value: String) {
        _uiState.update { it.copy(documentType = value) }
    }

    fun onDocumentNumberChange(value: String) {
        _uiState.update { it.copy(documentNumber = value) }
    }

    fun onDocumentSelected(file: File) {
        _uiState.update {
            it.copy(
                selectedFile = file,
                documentError = null,
                documentSuccess = null,
            )
        }
    }

    fun submitKyc() {
        val state = _uiState.value
        if (state.isLoading) return

        _uiState.update { it.copy(isLoading = true, submissionError = null) }
        viewModelScope.launch {
            try {
                val submission = kycRepository.submit(
                    KycSubmissionDto(
                        fullName = state.fullName.trim(),
                        nationality = state.nationality.trim(),
                        documentType = state.documentType.trim(),
                        documentNumber = state.documentNumber.trim(),
                    )
                )
                _uiState.update {
                    it.copy(
                        isLoading = false,
                        submitted = true,
                        status = submission.status,
                        submittedAt = submission.submittedAt,
                    )
                }
            } catch (t: Throwable) {
                if (t is CancellationException) throw t
                _uiState.update {
                    it.copy(
                        isLoading = false,
                        submissionError = AuthErrorMessages.from(t, "Failed to submit KYC."),
                    )
                }
            }
        }
    }

    fun uploadDocument() {
        val file = _uiState.value.selectedFile ?: return
        if (_uiState.value.isUploading) return

        // Client-side size validation (≤5 MB)
        if (file.length() > MAX_FILE_SIZE_BYTES) {
            _uiState.update {
                it.copy(documentError = "File must be 5 MB or smaller.")
            }
            return
        }

        // Client-side type validation (jpg/png/pdf only)
        val ext = file.extension.lowercase()
        if (ext !in ALLOWED_EXTENSIONS) {
            _uiState.update {
                it.copy(documentError = "Only JPG, PNG, and PDF files are accepted.")
            }
            return
        }

        _uiState.update { it.copy(isUploading = true, documentError = null) }
        viewModelScope.launch {
            try {
                val mimeType = when (ext) {
                    "jpg", "jpeg" -> "image/jpeg"
                    "png" -> "image/png"
                    "pdf" -> "application/pdf"
                    else -> "application/octet-stream"
                }
                val requestBody = file.asRequestBody(mimeType.toMediaTypeOrNull())
                val part = MultipartBody.Part.createFormData("document", file.name, requestBody)
                kycRepository.uploadDocument(part)
                _uiState.update {
                    it.copy(
                        isUploading = false,
                        documentSuccess = "Uploaded",
                        uploadComplete = true,
                    )
                }
            } catch (t: Throwable) {
                if (t is CancellationException) throw t
                _uiState.update {
                    it.copy(
                        isUploading = false,
                        documentError = AuthErrorMessages.from(t, "Upload failed."),
                    )
                }
            }
        }
    }

    private fun checkStatus() {
        viewModelScope.launch {
            try {
                val submission = kycRepository.status()
                _uiState.update {
                    it.copy(
                        status = submission.status,
                        submitted = true,
                        submittedAt = submission.submittedAt,
                    )
                }
            } catch (t: Throwable) {
                if (t is CancellationException) throw t
                // 404 = no submission yet, which is the expected initial state
            }
        }
    }

    private companion object {
        /** Maximum upload size: 5 MB (design D2). */
        const val MAX_FILE_SIZE_BYTES = 5L * 1024 * 1024

        /** Allowed file extensions (design D2: jpg/png/pdf). */
        val ALLOWED_EXTENSIONS = setOf("jpg", "jpeg", "png", "pdf")
    }
}
