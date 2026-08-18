package com.casino.app.ui.screens.kyc

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.casino.app.ui.screens.auth.AuthTextField
import com.casino.app.ui.theme.AccentGreen
import com.casino.app.ui.theme.BgDeep
import com.casino.app.ui.theme.BgSurface
import com.casino.app.ui.theme.StatusSuccess
import com.casino.app.ui.theme.TextMuted
import com.casino.app.ui.theme.TextSecondary

/**
 * KYC screen (task 6.3).
 *
 * Displays a KYC identity submission form and a document upload section. If a
 * submission already exists, only the status is shown (pending/approved/rejected).
 *
 * Client-side validation prevents uploads >5 MB and disallowed types (jpg/png/pdf)
 * before the network call (design D2 + spec requirement).
 *
 * @param onBack Invoked when the user taps the back action.
 */
@Composable
fun KycScreen(
    onBack: () -> Unit = {},
    viewModel: KycViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsStateWithLifecycle()

    KycScreenContent(
        uiState = uiState,
        onFullNameChange = viewModel::onFullNameChange,
        onNationalityChange = viewModel::onNationalityChange,
        onDocumentTypeChange = viewModel::onDocumentTypeChange,
        onDocumentNumberChange = viewModel::onDocumentNumberChange,
        onSubmitKyc = viewModel::submitKyc,
        onUploadDocument = viewModel::uploadDocument,
        onBack = onBack,
    )
}

@Composable
private fun KycScreenContent(
    uiState: KycUiState,
    onFullNameChange: (String) -> Unit,
    onNationalityChange: (String) -> Unit,
    onDocumentTypeChange: (String) -> Unit,
    onDocumentNumberChange: (String) -> Unit,
    onSubmitKyc: () -> Unit,
    onUploadDocument: () -> Unit,
    onBack: () -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(BgDeep)
            .verticalScroll(rememberScrollState())
            .imePadding()
            .padding(horizontal = 24.dp, vertical = 16.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            text = "KYC Verification",
            color = AccentGreen,
            fontSize = 24.sp,
            fontWeight = FontWeight.Bold,
            modifier = Modifier
                .fillMaxWidth()
                .padding(bottom = 24.dp),
        )

        // Status display (after submission or on load)
        if (uiState.status != null) {
            StatusCard(status = uiState.status, submittedAt = uiState.submittedAt)
            Spacer(modifier = Modifier.height(16.dp))
        }

        // Submission form (only when no existing submission)
        if (!uiState.submitted) {
            uiState.submissionError?.let { error ->
                Text(
                    text = error,
                    color = MaterialTheme.colorScheme.error,
                    fontSize = 13.sp,
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(bottom = 12.dp),
                )
            }

            AuthTextField(
                value = uiState.fullName,
                onValueChange = onFullNameChange,
                label = "Full Name",
                keyboardType = KeyboardType.Text,
            )
            Spacer(modifier = Modifier.height(12.dp))

            AuthTextField(
                value = uiState.nationality,
                onValueChange = onNationalityChange,
                label = "Nationality (e.g. ARG)",
                keyboardType = KeyboardType.Text,
            )
            Spacer(modifier = Modifier.height(12.dp))

            AuthTextField(
                value = uiState.documentType,
                onValueChange = onDocumentTypeChange,
                label = "Document Type (e.g. dni, passport)",
                keyboardType = KeyboardType.Text,
            )
            Spacer(modifier = Modifier.height(12.dp))

            AuthTextField(
                value = uiState.documentNumber,
                onValueChange = onDocumentNumberChange,
                label = "Document Number",
                keyboardType = KeyboardType.Text,
            )
            Spacer(modifier = Modifier.height(24.dp))

            Button(
                onClick = onSubmitKyc,
                enabled = !uiState.isLoading,
                modifier = Modifier
                    .fillMaxWidth()
                    .height(48.dp),
                shape = RoundedCornerShape(8.dp),
                colors = ButtonDefaults.buttonColors(containerColor = AccentGreen),
            ) {
                if (uiState.isLoading) {
                    CircularProgressIndicator(
                        modifier = Modifier.size(20.dp),
                        color = BgDeep,
                        strokeWidth = 2.dp,
                    )
                } else {
                    Text("Submit KYC", fontWeight = FontWeight.SemiBold)
                }
            }
        }

        // Document upload section (always available)
        if (uiState.submitted || uiState.status != null) {
            Spacer(modifier = Modifier.height(24.dp))

            Text(
                text = "Upload Documents",
                color = TextSecondary,
                fontSize = 16.sp,
                fontWeight = FontWeight.SemiBold,
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(bottom = 12.dp),
            )

            uiState.documentError?.let { error ->
                Text(
                    text = error,
                    color = MaterialTheme.colorScheme.error,
                    fontSize = 13.sp,
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(bottom = 8.dp),
                )
            }

            uiState.documentSuccess?.let { success ->
                Text(
                    text = success,
                    color = StatusSuccess,
                    fontSize = 13.sp,
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(bottom = 8.dp),
                )
            }

            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                OutlinedButton(
                    onClick = {
                        // In production, this would use ActivityResultContracts.OpenDocument
                        // For now, the ViewModel handles file selection via onDocumentSelected
                    },
                    modifier = Modifier.weight(1f),
                    shape = RoundedCornerShape(8.dp),
                ) {
                    Text(
                        text = uiState.selectedFile?.name ?: "Choose file",
                        color = TextSecondary,
                        fontSize = 14.sp,
                        maxLines = 1,
                    )
                }

                Spacer(modifier = Modifier.width(12.dp))

                Button(
                    onClick = onUploadDocument,
                    enabled = uiState.selectedFile != null && !uiState.isUploading,
                    modifier = Modifier.height(48.dp),
                    shape = RoundedCornerShape(8.dp),
                    colors = ButtonDefaults.buttonColors(containerColor = AccentGreen),
                ) {
                    if (uiState.isUploading) {
                        CircularProgressIndicator(
                            modifier = Modifier.size(16.dp),
                            color = BgDeep,
                            strokeWidth = 2.dp,
                        )
                    } else {
                        Text("Upload", fontWeight = FontWeight.SemiBold)
                    }
                }
            }
        }

        Spacer(modifier = Modifier.height(16.dp))

        OutlinedButton(
            onClick = onBack,
            modifier = Modifier.fillMaxWidth(),
            shape = RoundedCornerShape(8.dp),
        ) {
            Text("Back to Profile", color = TextSecondary, fontSize = 14.sp)
        }
    }
}

@Composable
private fun StatusCard(status: String, submittedAt: String?) {
    val statusColor = when (status) {
        "pending" -> com.casino.app.ui.theme.StatusWarning
        "approved" -> StatusSuccess
        "rejected" -> com.casino.app.ui.theme.StatusError
        else -> TextMuted
    }

    Column(
        modifier = Modifier
            .fillMaxWidth()
            .background(BgSurface, RoundedCornerShape(8.dp))
            .padding(16.dp),
    ) {
        Text(
            text = "KYC Status",
            color = TextSecondary,
            fontSize = 14.sp,
            fontWeight = FontWeight.SemiBold,
        )
        Spacer(modifier = Modifier.height(4.dp))
        Text(
            text = status.replaceFirstChar { it.uppercase() },
            color = statusColor,
            fontSize = 18.sp,
            fontWeight = FontWeight.Bold,
        )
        submittedAt?.let {
            Text(
                text = "Submitted: $it",
                color = TextMuted,
                fontSize = 12.sp,
                modifier = Modifier.padding(top = 4.dp),
            )
        }
    }
}
