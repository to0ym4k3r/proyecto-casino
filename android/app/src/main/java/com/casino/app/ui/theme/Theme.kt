/**
 * Material3 theme configuration for the Casino application.
 *
 * Provides a dark color scheme with casino-inspired accent colors,
 * applies the custom [CasinoTypography] and [CasinoShapes], and
 * wraps content in a [CasinoTheme] composable.
 *
 * @author Casino Team
 * @since 2026-08-03
 */
package com.casino.app.ui.theme

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable

/**
 * Dark color scheme for the Casino application.
 *
 * Maps Material3 color slots to the casino palette defined in
 * [Color.kt], using deep backgrounds, green accents, and muted text.
 */
private val CasinoDarkColorScheme = darkColorScheme(
    primary = AccentGreen,
    onPrimary = TextPrimary,
    primaryContainer = AccentGreenDark,
    onPrimaryContainer = TextPrimary,
    secondary = AccentBlue,
    onSecondary = TextPrimary,
    secondaryContainer = BgElevated,
    onSecondaryContainer = TextPrimary,
    tertiary = AccentGold,
    onTertiary = BgDeep,
    background = BgDeep,
    onBackground = TextPrimary,
    surface = BgPrimary,
    onSurface = TextPrimary,
    surfaceVariant = BgElevated,
    onSurfaceVariant = TextSecondary,
    error = AccentRed,
    onError = TextPrimary,
    outline = TextMuted,
    outlineVariant = TextDimmed
)

/**
 * Root theme composable for the Casino application.
 *
 * Applies the [CasinoDarkColorScheme], [CasinoTypography], and
 * [CasinoShapes] to all descendant composables via [MaterialTheme].
 *
 * @param content The composable content tree that will receive the theme.
 */
@Composable
fun CasinoTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = CasinoDarkColorScheme,
        typography = CasinoTypography,
        shapes = CasinoShapes,
        content = content
    )
}
