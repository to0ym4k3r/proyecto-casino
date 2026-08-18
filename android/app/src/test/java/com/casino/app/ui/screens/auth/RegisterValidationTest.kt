package com.casino.app.ui.screens.auth

import java.time.Instant
import java.time.LocalDate
import java.util.TimeZone
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * RED (5.7): pure field validation for the register form — required fields,
 * email format, username length and the 18+ age gate with an injectable
 * reference date (spec: "over 18" rule; spec scenario: rejected without
 * network calls).
 */
class RegisterValidationTest {

    private val today: LocalDate = LocalDate.of(2026, 8, 15)

    @Test
    fun `valid inputs produce no errors`() {
        val errors = RegisterValidation.validate(
            username = "alice",
            email = "alice@casino.dev",
            password = "s3cret!",
            firstName = "Alice",
            lastName = "Wonder",
            dateOfBirth = "1998-04-12",
            today = today,
        )

        assertTrue(errors.isEmpty)
        assertNull(errors.email)
        assertNull(errors.dateOfBirth)
    }

    @Test
    fun `malformed emails are rejected`() {
        for (email in listOf("not-an-email", "a@b", "user@domain.c", "user@.com", "user@domain.")) {
            val errors = RegisterValidation.validate(
                username = "alice",
                email = email,
                password = "s3cret!",
                firstName = "Alice",
                lastName = "Wonder",
                dateOfBirth = "1998-04-12",
                today = today,
            )

            assertEquals("Enter a valid email address.", errors.email)
        }
    }

    @Test
    fun `blank email is required`() {
        val errors = RegisterValidation.validate(
            username = "alice",
            email = "   ",
            password = "s3cret!",
            firstName = "Alice",
            lastName = "Wonder",
            dateOfBirth = "1998-04-12",
            today = today,
        )

        assertEquals("Email is required.", errors.email)
    }

    @Test
    fun `underage date of birth is rejected`() {
        val errors = RegisterValidation.validate(
            username = "kid",
            email = "kid@casino.dev",
            password = "s3cret!",
            firstName = "Kid",
            lastName = "Test",
            dateOfBirth = "2008-08-16", // 17y364d on 2026-08-15
            today = today,
        )

        assertEquals("You must be at least 18 years old.", errors.dateOfBirth)
    }

    @Test
    fun `exactly 18 years old is accepted`() {
        val errors = RegisterValidation.validate(
            username = "adult",
            email = "adult@casino.dev",
            password = "s3cret!",
            firstName = "New",
            lastName = "Adult",
            dateOfBirth = "2008-08-15", // exactly 18 on 2026-08-15
            today = today,
        )

        assertNull(errors.dateOfBirth)
    }

    @Test
    fun `future date of birth is rejected`() {
        val errors = RegisterValidation.validate(
            username = "alice",
            email = "alice@casino.dev",
            password = "s3cret!",
            firstName = "Alice",
            lastName = "Wonder",
            dateOfBirth = "2030-01-01",
            today = today,
        )

        assertEquals("Date of birth cannot be in the future.", errors.dateOfBirth)
    }

    @Test
    fun `unparseable date of birth shows the expected format`() {
        val errors = RegisterValidation.validate(
            username = "alice",
            email = "alice@casino.dev",
            password = "s3cret!",
            firstName = "Alice",
            lastName = "Wonder",
            dateOfBirth = "01/02/1998",
            today = today,
        )

        assertEquals("Enter your date of birth as YYYY-MM-DD.", errors.dateOfBirth)
    }

    @Test
    fun `short username is rejected`() {
        val errors = RegisterValidation.validate(
            username = "ab",
            email = "alice@casino.dev",
            password = "s3cret!",
            firstName = "Alice",
            lastName = "Wonder",
            dateOfBirth = "1998-04-12",
            today = today,
        )

        assertEquals("Username must be at least 3 characters.", errors.username)
    }

    @Test
    fun `blank password is required`() {
        val errors = RegisterValidation.validate(
            username = "alice",
            email = "alice@casino.dev",
            password = "",
            firstName = "Alice",
            lastName = "Wonder",
            dateOfBirth = "1998-04-12",
            today = today,
        )

        assertEquals("Password is required.", errors.password)
    }

    @Test
    fun `empty form collects every required error`() {
        val errors = RegisterValidation.validate(
            username = "",
            email = "",
            password = "",
            firstName = "",
            lastName = "",
            dateOfBirth = "",
            today = today,
        )

        assertFalse(errors.isEmpty)
        assertEquals("Username is required.", errors.username)
        assertEquals("Email is required.", errors.email)
        assertEquals("Password is required.", errors.password)
        assertEquals("First name is required.", errors.firstName)
        assertEquals("Last name is required.", errors.lastName)
        assertEquals("Date of birth is required.", errors.dateOfBirth)
    }
}

/**
 * RED (finding 3 — DatePicker UTC off-by-one): Material3's
 * `DatePickerState.selectedDateMillis` is the UTC midnight of the picked day.
 * Converting with the LOCAL zone shifts the day back on negative-offset
 * devices (the Americas), corrupting `date_of_birth` and loosening the client
 * 18+ gate by one day. The conversion must be zone-independent (UTC).
 */
class RegisterDatePickerFormatTest {

    @Test
    fun `formats the picked UTC day, not the local-shifted day, in a negative-offset zone`() {
        withTimeZone("America/New_York") {
            val millis = Instant.parse("2000-01-02T00:00:00Z").toEpochMilli()
            assertEquals("2000-01-02", formatSelectedDateMillis(millis))
        }
    }

    @Test
    fun `formats the picked UTC day in a positive-offset zone`() {
        withTimeZone("Asia/Tokyo") {
            val millis = Instant.parse("2000-01-02T00:00:00Z").toEpochMilli()
            assertEquals("2000-01-02", formatSelectedDateMillis(millis))
        }
    }

    @Test
    fun `formats a mid-day selection without shifting the day`() {
        withTimeZone("America/New_York") {
            val millis = Instant.parse("2000-06-15T12:00:00Z").toEpochMilli()
            assertEquals("2000-06-15", formatSelectedDateMillis(millis))
        }
    }

    private fun withTimeZone(id: String, block: () -> Unit) {
        val original = TimeZone.getDefault()
        try {
            TimeZone.setDefault(TimeZone.getTimeZone(id))
            block()
        } finally {
            TimeZone.setDefault(original)
        }
    }
}
