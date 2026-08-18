package com.casino.app.data.token

import androidx.datastore.preferences.core.PreferenceDataStoreFactory
import com.casino.app.data.api.dto.TokenPair
import java.io.File
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.TestCoroutineScheduler
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

/**
 * Temp-file Preferences DataStore tests for [TokenStore] (task 5.3).
 *
 * Prove the spec's token-storage requirements (android-auth): access/refresh
 * tokens persist in DataStore, session state restores from stored tokens after
 * a restart, and clearing the store makes the auth gate show login.
 *
 * Each store owns its own [CoroutineScope]; cancelling it simulates the app
 * process ending (DataStore 1.1.x allows one active instance per file, so a
 * "restart" must cancel the previous scope before opening a new instance).
 */
@OptIn(ExperimentalCoroutinesApi::class)
class TokenStoreTest {

    /** A store bound to its lifecycle scope so tests can cancel it. */
    private class TestStore(val scope: CoroutineScope, val store: TokenStore)

    private fun TestScope.newTestStore(file: File): TestStore {
        val scope = CoroutineScope(UnconfinedTestDispatcher(testScheduler))
        val dataStore = PreferenceDataStoreFactory.create(
            scope = scope,
            produceFile = { file }
        )
        return TestStore(scope, TokenStore(dataStore))
    }

    /**
     * A unique temp-file path that does NOT exist yet: DataStore creates the
     * file on its first write.
     */
    private fun tempFile(): File =
        File.createTempFile("token_store_test", ".preferences_pb").apply { delete() }

    @Test
    fun `empty store yields no tokens`() = runTest {
        val file = tempFile()
        try {
            val testStore = newTestStore(file)
            try {
                assertNull("no access token on a fresh store", testStore.store.accessToken.first())
                assertNull("no refresh token on a fresh store", testStore.store.refreshToken.first())
                assertNull("no token pair on a fresh store", testStore.store.tokens.first())
            } finally {
                testStore.scope.cancel()
            }
        } finally {
            file.delete()
        }
    }

    @Test
    fun `saved tokens persist across store instances`() = runTest {
        val file = tempFile()
        try {
            val firstProcess = newTestStore(file)
            try {
                firstProcess.store.saveTokens(TokenPair(accessToken = "access-1", refreshToken = "refresh-1"))
            } finally {
                // The first app process ends, releasing the file.
                firstProcess.scope.cancel()
            }

            // A second store over the same file simulates an app restart:
            // session state must restore from the stored tokens.
            val restarted = newTestStore(file)
            try {
                assertEquals("access-1", restarted.store.accessToken.first())
                assertEquals("refresh-1", restarted.store.refreshToken.first())
                assertEquals(TokenPair("access-1", "refresh-1"), restarted.store.tokens.first())
            } finally {
                restarted.scope.cancel()
            }
        } finally {
            file.delete()
        }
    }

    @Test
    fun `clear removes tokens so the auth gate shows login`() = runTest {
        val file = tempFile()
        try {
            val testStore = newTestStore(file)
            try {
                testStore.store.saveTokens(TokenPair(accessToken = "access-2", refreshToken = "refresh-2"))

                testStore.store.clear()

                assertNull(testStore.store.accessToken.first())
                assertNull(testStore.store.refreshToken.first())
                assertNull(testStore.store.tokens.first())
            } finally {
                testStore.scope.cancel()
            }
        } finally {
            file.delete()
        }
    }

    @Test
    fun `tokens flow tracks save then clear`() = runTest {
        val file = tempFile()
        try {
            val testStore = newTestStore(file)
            try {
                assertNull(testStore.store.tokens.first())

                testStore.store.saveTokens(TokenPair("access-3", "refresh-3"))
                assertEquals(TokenPair("access-3", "refresh-3"), testStore.store.tokens.first())

                testStore.store.clear()
                assertNull(testStore.store.tokens.first())
            } finally {
                testStore.scope.cancel()
            }
        } finally {
            file.delete()
        }
    }
}
