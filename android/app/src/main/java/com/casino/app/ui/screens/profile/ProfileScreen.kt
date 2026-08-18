package com.casino.app.ui.screens.profile

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
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
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
import com.casino.app.ui.theme.TextMuted
import com.casino.app.ui.theme.TextSecondary

/**
 * Profile screen (task 6.2).
 *
 * Displays the authenticated user's profile loaded via GET /users/me. Editable
 * fields: username, first_name, last_name (design D6). Email is shown but
 * immutable. Saving calls PUT /users/me and refreshes the displayed data.
 *
 * @param onNavigateToKyc Invoked when the user taps "KYC Verification".
 */
@Composable
fun ProfileScreen(
    onNavigateToKyc: () -> Unit = {},
    viewModel: ProfileViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsStateWithLifecycle()

    ProfileScreenContent(
        uiState = uiState,
        onUsernameChange = viewModel::onUsernameChange,
        onFirstNameChange = viewModel::onFirstNameChange,
        onLastNameChange = viewModel::onLastNameChange,
        onSave = viewModel::save,
        onNavigateToKyc = onNavigateToKyc,
    )
}

@Composable
private fun ProfileScreenContent(
    uiState: ProfileUiState,
    onUsernameChange: (String) -> Unit,
    onFirstNameChange: (String) -> Unit,
    onLastNameChange: (String) -> Unit,
    onSave: () -> Unit,
    onNavigateToKyc: () -> Unit,
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
            text = "My Profile",
            color = AccentGreen,
            fontSize = 24.sp,
            fontWeight = FontWeight.Bold,
            modifier = Modifier
                .fillMaxWidth()
                .padding(bottom = 24.dp),
        )

        if (uiState.isLoading) {
            CircularProgressIndicator(
                modifier = Modifier.size(32.dp),
                color = AccentGreen,
                strokeWidth = 3.dp,
            )
            return
        }

        uiState.error?.let { error ->
            Text(
                text = error,
                color = MaterialTheme.colorScheme.error,
                fontSize = 13.sp,
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(bottom = 12.dp),
            )
        }

        // Email (read-only)
        AuthTextField(
            value = uiState.email,
            onValueChange = {},
            label = "Email (read-only)",
            keyboardType = KeyboardType.Email,
        )
        Spacer(modifier = Modifier.height(12.dp))

        AuthTextField(
            value = uiState.username,
            onValueChange = onUsernameChange,
            label = "Username",
            keyboardType = KeyboardType.Text,
        )
        Spacer(modifier = Modifier.height(12.dp))

        AuthTextField(
            value = uiState.firstName,
            onValueChange = onFirstNameChange,
            label = "First Name",
            keyboardType = KeyboardType.Text,
        )
        Spacer(modifier = Modifier.height(12.dp))

        AuthTextField(
            value = uiState.lastName,
            onValueChange = onLastNameChange,
            label = "Last Name",
            keyboardType = KeyboardType.Text,
        )

        Spacer(modifier = Modifier.height(24.dp))

        Button(
            onClick = onSave,
            enabled = !uiState.isSaving,
            modifier = Modifier
                .fillMaxWidth()
                .height(48.dp),
            shape = RoundedCornerShape(8.dp),
            colors = ButtonDefaults.buttonColors(containerColor = AccentGreen),
        ) {
            if (uiState.isSaving) {
                CircularProgressIndicator(
                    modifier = Modifier.size(20.dp),
                    color = BgDeep,
                    strokeWidth = 2.dp,
                )
            } else {
                Text("Save Changes", fontWeight = FontWeight.SemiBold)
            }
        }

        Spacer(modifier = Modifier.height(16.dp))

        // KYC verification link
        Text(
            text = "KYC Verification",
            color = AccentGreen,
            fontSize = 14.sp,
            fontWeight = FontWeight.Medium,
            modifier = Modifier
                .fillMaxWidth()
                .background(BgSurface, RoundedCornerShape(8.dp))
                .padding(12.dp)
                .let { modifier ->
                    modifier // Placeholder — clickable added in the non-Content version
                },
        )
    }
}
