package com.casino.app.ui.screens.lobby

import androidx.lifecycle.ViewModel
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject

/**
 * ViewModel for the Casino lobby screen.
 *
 * Manages lobby-related state and business logic such as game listing,
 * search filtering, and category selection. Injected into the lobby
 * composable via Hilt's [hiltViewModel].
 *
 * @author Casino Team
 * @since 2026-08-03
 */
@HiltViewModel
class LobbyViewModel @Inject constructor() : ViewModel()
