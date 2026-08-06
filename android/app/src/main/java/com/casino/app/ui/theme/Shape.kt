/**
 * Shape definitions for the Casino application theme.
 *
 * Defines rounded corner shapes at four size levels (small through extraLarge)
 * to maintain consistent corner radii across Material3 components.
 *
 * @author Casino Team
 * @since 2026-08-03
 */
package com.casino.app.ui.theme

import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Shapes
import androidx.compose.ui.unit.dp

/** Shape scale used by Material3 components throughout the Casino application. */
val CasinoShapes = Shapes(
    small = RoundedCornerShape(6.dp),
    medium = RoundedCornerShape(8.dp),
    large = RoundedCornerShape(12.dp),
    extraLarge = RoundedCornerShape(16.dp)
)
