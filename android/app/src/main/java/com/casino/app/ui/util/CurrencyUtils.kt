package com.casino.app.ui.util

/**
 * Format integer cents as a dollar string (e.g. 50000 → "$500.00", -50 → "$-0.50").
 */
fun formatCentsAsDollars(cents: Long): String {
    val sign = if (cents < 0) "-" else ""
    val abs = kotlin.math.abs(cents)
    val dollars = abs / 100
    val centsRemainder = abs % 100
    return "$sign$$dollars.${centsRemainder.toString().padStart(2, '0')}"
}
