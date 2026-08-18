package com.casino.app

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.TestDispatcher
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.setMain
import org.junit.rules.TestWatcher
import org.junit.runner.Description

/**
 * JUnit rule that swaps [Dispatchers.Main] for a test dispatcher so ViewModels
 * (whose [androidx.lifecycle.viewModelScope] runs on Main.immediate) execute
 * deterministically in unit tests.
 *
 * The default is a [StandardTestDispatcher]: coroutines are queued until the
 * scheduler is advanced, which lets tests observe the INITIAL state of a
 * StateFlow (e.g. the gate's Splash) before upstream emissions land. Tests must
 * run inside `runTest(mainDispatcherRule.testDispatcher)` so the rule's
 * scheduler and the test scheduler are the same instance and
 * `advanceUntilIdle()` drives ViewModel coroutines too.
 */
@OptIn(ExperimentalCoroutinesApi::class)
class MainDispatcherRule(
    val testDispatcher: TestDispatcher = StandardTestDispatcher(),
) : TestWatcher() {
    override fun starting(description: Description) {
        Dispatchers.setMain(testDispatcher)
    }

    override fun finished(description: Description) {
        Dispatchers.resetMain()
    }
}
