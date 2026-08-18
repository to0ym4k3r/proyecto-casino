package com.casino.app.ui.screens.auth

import java.time.LocalDate
import java.time.Period
import java.time.format.DateTimeFormatter
import java.time.format.DateTimeParseException

/** Per-field inline errors for the register form; [isEmpty] = the form is valid. */
data class RegisterFieldErrors(
    val username: String? = null,
    val email: String? = null,
    val password: String? = null,
    val firstName: String? = null,
    val lastName: String? = null,
    val dateOfBirth: String? = null,
) {
    val isEmpty: Boolean
        get() = username == null && email == null && password == null &&
            firstName == null && lastName == null && dateOfBirth == null
}

/**
 * Pure validation rules for the register form (task 5.7). No Android or
 * network dependencies so the rules are directly unit-testable; [today] is
 * injectable for deterministic age-gate tests.
 */
object RegisterValidation {

    private val EMAIL_REGEX = Regex("^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\\.[A-Za-z]{2,}\$")

    fun validate(
        username: String,
        email: String,
        password: String,
        firstName: String,
        lastName: String,
        dateOfBirth: String,
        today: LocalDate = LocalDate.now(),
    ): RegisterFieldErrors {
        val trimmedUsername = username.trim()
        val trimmedEmail = email.trim()
        val trimmedFirst = firstName.trim()
        val trimmedLast = lastName.trim()
        val trimmedDob = dateOfBirth.trim()

        return RegisterFieldErrors(
            username = when {
                trimmedUsername.isEmpty() -> "Username is required."
                trimmedUsername.length < 3 -> "Username must be at least 3 characters."
                else -> null
            },
            email = when {
                trimmedEmail.isEmpty() -> "Email is required."
                !EMAIL_REGEX.matches(trimmedEmail) -> "Enter a valid email address."
                else -> null
            },
            password = if (password.isEmpty()) "Password is required." else null,
            firstName = if (trimmedFirst.isEmpty()) "First name is required." else null,
            lastName = if (trimmedLast.isEmpty()) "Last name is required." else null,
            dateOfBirth = validateDateOfBirth(trimmedDob, today),
        )
    }

    private fun validateDateOfBirth(dob: String, today: LocalDate): String? {
        if (dob.isEmpty()) return "Date of birth is required."
        val parsed = try {
            LocalDate.parse(dob, DateTimeFormatter.ISO_LOCAL_DATE)
        } catch (_: DateTimeParseException) {
            return "Enter your date of birth as YYYY-MM-DD."
        }
        return when {
            parsed.isAfter(today) -> "Date of birth cannot be in the future."
            Period.between(parsed, today).years < 18 -> "You must be at least 18 years old."
            else -> null
        }
    }
}
