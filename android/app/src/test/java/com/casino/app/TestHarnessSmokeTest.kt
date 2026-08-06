package com.casino.app

import app.cash.turbine.test
import io.mockk.every
import io.mockk.mockk
import io.mockk.verify
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Smoke tests that verify the Android unit test harness is wired correctly.
 *
 * These tests exercise JUnit4, MockK, kotlinx-coroutines-test and Turbine on
 * minimal self-contained fixtures. They prove the tooling resolves, compiles
 * and runs locally so strict TDD can be applied on the Android module.
 *
 * @author Casino Team
 * @since 2026-08-05
 */
class TestHarnessSmokeTest {

    /** Pure function used to check plain JUnit4 assertions. */
    private fun casinoJackpot(bet: Int, multiplier: Int): Int = bet * multiplier

    /** Minimal collaborator used to check MockK stubbing and verification. */
    private interface BettingService {
        fun resolve(bet: Int): Int
    }

    @Test
    fun `junit4 pure function assertions run`() {
        val payout = casinoJackpot(bet = 10, multiplier = 3)

        assertEquals(30, payout)
        assertTrue("payout must be positive", payout > 0)
    }

    @Test
    fun `mockk can stub and verify a collaborator`() {
        val service: BettingService = mockk()
        every { service.resolve(10) } returns 40

        val result = service.resolve(10)

        assertEquals(40, result)
        verify { service.resolve(10) }
    }

    @Test
    fun `coroutines-test and turbine can collect a flow`() = runTest {
        val winnings: Flow<Int> = flowOf(1, 2, 3)

        winnings.test {
            assertEquals(1, awaitItem())
            assertEquals(2, awaitItem())
            assertEquals(3, awaitItem())
            awaitComplete()
        }
    }
}
