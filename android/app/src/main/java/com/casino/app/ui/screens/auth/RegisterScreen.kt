package com.casino.app.ui.screens.auth

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DatePicker
import androidx.compose.material3.DatePickerDialog
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberDatePickerState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.casino.app.ui.theme.AccentGreen
import com.casino.app.ui.theme.BgDeep
import com.casino.app.ui.theme.BgSurface
import com.casino.app.ui.theme.StatusSuccess
import com.casino.app.ui.theme.TextMuted
import com.casino.app.ui.theme.TextPrimary
import com.casino.app.ui.theme.TextSecondary
import java.time.Instant
import java.time.LocalDate
import java.time.ZoneOffset
import java.time.format.DateTimeFormatter

/**
 * Register screen (task 5.7).
 *
 * Spec scenario: "GIVEN invalid register data WHEN submitting register THEN
 * inline validation errors shown and no network call" — field errors render
 * next to their inputs; on success the screen guides the user to the email
 * verification step (registration does not log the user in).
 *
 * @param onNavigateToVerify Invoked after successful registration to start
 *   email verification.
 * @param onNavigateToLogin Invoked when the user goes back to sign in.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun RegisterScreen(
    onNavigateToVerify: () -> Unit,
    onNavigateToLogin: () -> Unit,
    viewModel: RegisterViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsStateWithLifecycle()
    var showDatePicker by remember { mutableStateOf(false) }

    if (showDatePicker) {
        val datePickerState = rememberDatePickerState()
        DatePickerDialog(
            onDismissRequest = { showDatePicker = false },
            confirmButton = {
                TextButton(
                    onClick = {
                        datePickerState.selectedDateMillis?.let { millis ->
                            viewModel.onDateOfBirthChange(formatSelectedDateMillis(millis))
                        }
                        showDatePicker = false
                    },
                ) {
                    Text("OK", color = AccentGreen)
                }
            },
            dismissButton = {
                TextButton(onClick = { showDatePicker = false }) {
                    Text("Cancel", color = TextSecondary)
                }
            },
        ) {
            DatePicker(state = datePickerState)
        }
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(BgDeep)
            .verticalScroll(rememberScrollState())
            .imePadding()
            .padding(horizontal = 24.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Spacer(modifier = Modifier.height(48.dp))
        Text(
            text = "Create account",
            color = TextPrimary,
            fontSize = 24.sp,
            fontWeight = FontWeight.Bold,
        )
        Text(
            text = "You must be at least 18 years old",
            color = TextSecondary,
            fontSize = 13.sp,
            modifier = Modifier.padding(top = 4.dp, bottom = 20.dp),
        )

        if (uiState.registered) {
            Text(
                text = "Account created. Verify your email to sign in.",
                color = StatusSuccess,
                fontSize = 14.sp,
                fontWeight = FontWeight.Medium,
                modifier = Modifier
                    .fillMaxWidth()
                    .background(BgSurface, RoundedCornerShape(8.dp))
                    .padding(12.dp),
            )
            Spacer(modifier = Modifier.height(16.dp))
            Button(
                onClick = onNavigateToVerify,
                modifier = Modifier
                    .fillMaxWidth()
                    .height(48.dp),
                shape = RoundedCornerShape(8.dp),
                colors = ButtonDefaults.buttonColors(containerColor = AccentGreen),
            ) {
                Text("Verify email", fontWeight = FontWeight.SemiBold)
            }
            TextButton(onClick = onNavigateToLogin, modifier = Modifier.padding(top = 8.dp)) {
                Text("Back to login", color = TextSecondary, fontSize = 14.sp)
            }
            return
        }

        RegisterField(
            value = uiState.username,
            onValueChange = viewModel::onUsernameChange,
            label = "Username",
            error = uiState.fieldErrors.username,
        )
        RegisterField(
            value = uiState.email,
            onValueChange = viewModel::onEmailChange,
            label = "Email",
            keyboardType = KeyboardType.Email,
            error = uiState.fieldErrors.email,
        )
        RegisterField(
            value = uiState.firstName,
            onValueChange = viewModel::onFirstNameChange,
            label = "First name",
            error = uiState.fieldErrors.firstName,
        )
        RegisterField(
            value = uiState.lastName,
            onValueChange = viewModel::onLastNameChange,
            label = "Last name",
            error = uiState.fieldErrors.lastName,
        )
        RegisterField(
            value = uiState.password,
            onValueChange = viewModel::onPasswordChange,
            label = "Password",
            keyboardType = KeyboardType.Password,
            isPassword = true,
            error = uiState.fieldErrors.password,
        )

        OutlinedTextField(
            value = uiState.dateOfBirth,
            onValueChange = viewModel::onDateOfBirthChange,
            label = { Text("Date of birth (YYYY-MM-DD)", color = TextMuted) },
            singleLine = true,
            isError = uiState.fieldErrors.dateOfBirth != null,
            keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(keyboardType = KeyboardType.Text),
            supportingText = uiState.fieldErrors.dateOfBirth?.let { error ->
                { Text(error, color = MaterialTheme.colorScheme.error) }
            },
            shape = RoundedCornerShape(8.dp),
            colors = OutlinedTextFieldDefaults.colors(
                focusedBorderColor = AccentGreen,
                unfocusedBorderColor = BgSurface,
                focusedContainerColor = BgSurface,
                unfocusedContainerColor = BgSurface,
                cursorColor = AccentGreen,
                focusedTextColor = TextPrimary,
                unfocusedTextColor = TextPrimary,
            ),
            trailingIcon = {
                TextButton(onClick = { showDatePicker = true }) {
                    Text("Pick", color = AccentGreen)
                }
            },
            modifier = Modifier.fillMaxWidth(),
        )

        uiState.error?.let { error ->
            Text(
                text = error,
                color = MaterialTheme.colorScheme.error,
                fontSize = 13.sp,
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(top = 12.dp),
            )
        }

        Spacer(modifier = Modifier.height(24.dp))

        Button(
            onClick = viewModel::submit,
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
                Text("Create account", fontWeight = FontWeight.SemiBold)
            }
        }

        TextButton(onClick = onNavigateToLogin, modifier = Modifier.padding(top = 8.dp)) {
            Text(text = "Sign in instead", color = TextSecondary, fontSize = 14.sp)
        }
        Spacer(modifier = Modifier.height(24.dp))
    }
}

@Composable
private fun RegisterField(
    value: String,
    onValueChange: (String) -> Unit,
    label: String,
    error: String?,
    keyboardType: KeyboardType = KeyboardType.Text,
    isPassword: Boolean = false,
) {
    OutlinedTextField(
        value = value,
        onValueChange = onValueChange,
        label = { Text(label, color = TextMuted) },
        singleLine = true,
        isError = error != null,
        keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(keyboardType = keyboardType),
        visualTransformation = if (isPassword) PasswordVisualTransformation() else androidx.compose.ui.text.input.VisualTransformation.None,
        supportingText = error?.let { message ->
            { Text(message, color = MaterialTheme.colorScheme.error) }
        },
        shape = RoundedCornerShape(8.dp),
        colors = OutlinedTextFieldDefaults.colors(
            focusedBorderColor = AccentGreen,
            unfocusedBorderColor = BgSurface,
            focusedContainerColor = BgSurface,
            unfocusedContainerColor = BgSurface,
            cursorColor = AccentGreen,
            focusedTextColor = TextPrimary,
            unfocusedTextColor = TextPrimary,
        ),
        modifier = Modifier
            .fillMaxWidth()
            .padding(bottom = 12.dp),
    )
}

/**
 * Formats the Material3 date-picker selection — [DatePickerState.selectedDateMillis],
 * the UTC midnight of the picked day — as ISO_LOCAL_DATE. Converted in UTC:
 * the local zone shifts the day back on negative-offset devices (the
 * Americas), which corrupts `date_of_birth` and loosens the client 18+ gate
 * by one day (finding: DatePicker UTC off-by-one).
 */
internal fun formatSelectedDateMillis(millis: Long): String =
    Instant.ofEpochMilli(millis)
        .atZone(ZoneOffset.UTC)
        .toLocalDate()
        .format(DateTimeFormatter.ISO_LOCAL_DATE)
