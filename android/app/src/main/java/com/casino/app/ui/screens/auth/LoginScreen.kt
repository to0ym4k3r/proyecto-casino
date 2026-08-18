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
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
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

/**
 * Login screen (task 5.6).
 *
 * Backed by [LoginViewModel]: wrong credentials render as an inline error with
 * NO navigation (the auth gate reacts to a successful session change), the
 * submit button disables while loading, and a verified banner shows when the
 * verify screen handed back an "email verified" flag.
 *
 * @param onNavigateToRegister Invoked when the user taps "Create account".
 */
@Composable
fun LoginScreen(
    onNavigateToRegister: () -> Unit,
    viewModel: LoginViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsStateWithLifecycle()
    val verifiedBanner by viewModel.verifiedBanner.collectAsStateWithLifecycle()

    LoginScreenContent(
        uiState = uiState,
        verifiedBanner = verifiedBanner,
        onEmailChange = viewModel::onEmailChange,
        onPasswordChange = viewModel::onPasswordChange,
        onLogin = viewModel::login,
        onBannerDismiss = viewModel::consumeVerifiedBanner,
        onNavigateToRegister = onNavigateToRegister,
    )
}

@Composable
private fun LoginScreenContent(
    uiState: LoginUiState,
    verifiedBanner: Boolean,
    onEmailChange: (String) -> Unit,
    onPasswordChange: (String) -> Unit,
    onLogin: () -> Unit,
    onBannerDismiss: () -> Unit,
    onNavigateToRegister: () -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(BgDeep)
            .verticalScroll(rememberScrollState())
            .imePadding()
            .padding(horizontal = 24.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center,
    ) {
        Text(
            text = "CASINO",
            color = AccentGreen,
            fontSize = 32.sp,
            fontWeight = FontWeight.ExtraBold,
        )
        Text(
            text = "Sign in to continue",
            color = TextSecondary,
            fontSize = 14.sp,
            modifier = Modifier.padding(top = 4.dp, bottom = 24.dp),
        )

        if (verifiedBanner) {
            Text(
                text = "Email verified — you can now sign in.",
                color = StatusSuccess,
                fontSize = 13.sp,
                fontWeight = FontWeight.Medium,
                modifier = Modifier
                    .fillMaxWidth()
                    .background(BgSurface, RoundedCornerShape(8.dp))
                    .padding(12.dp),
            )
            Spacer(modifier = Modifier.height(16.dp))
        }

        AuthTextField(
            value = uiState.email,
            onValueChange = onEmailChange,
            label = "Email",
            keyboardType = KeyboardType.Email,
        )
        Spacer(modifier = Modifier.height(12.dp))
        AuthTextField(
            value = uiState.password,
            onValueChange = onPasswordChange,
            label = "Password",
            keyboardType = KeyboardType.Password,
            isPassword = true,
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
            onClick = onLogin,
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
                Text("Sign in", fontWeight = FontWeight.SemiBold)
            }
        }

        TextButton(onClick = onNavigateToRegister, modifier = Modifier.padding(top = 8.dp)) {
            Text(
                text = "Create account",
                color = TextSecondary,
                fontSize = 14.sp,
            )
        }
    }
}

/** Shared single-line outlined field matching the app's casino theme. */
@Composable
internal fun AuthTextField(
    value: String,
    onValueChange: (String) -> Unit,
    label: String,
    keyboardType: KeyboardType,
    isPassword: Boolean = false,
) {
    OutlinedTextField(
        value = value,
        onValueChange = onValueChange,
        label = { Text(label, color = TextMuted) },
        singleLine = true,
        isError = false,
        keyboardOptions = KeyboardOptions(keyboardType = keyboardType),
        visualTransformation = if (isPassword) PasswordVisualTransformation() else androidx.compose.ui.text.input.VisualTransformation.None,
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
        modifier = Modifier.fillMaxWidth(),
    )
}
