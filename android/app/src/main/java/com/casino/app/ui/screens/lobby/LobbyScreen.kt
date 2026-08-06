/**
 * Lobby screen implementation for the Casino application.
 *
 * Displays a searchable, filterable grid of available games. Includes
 * a header, pill-shaped search bar, category filter chips, and a two-column
 * game card grid. Uses [LobbyViewModel] via Hilt for state management.
 *
 * @author Casino Team
 * @since 2026-08-03
 */
package com.casino.app.ui.screens.lobby

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FilterChipDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import com.casino.app.ui.theme.AccentGreen
import com.casino.app.ui.theme.AccentRed
import com.casino.app.ui.theme.BgDeep
import com.casino.app.ui.theme.BgElevated
import com.casino.app.ui.theme.BgPrimary
import com.casino.app.ui.theme.BgSurface
import com.casino.app.ui.theme.TextMuted
import com.casino.app.ui.theme.TextPrimary
import com.casino.app.ui.theme.TextSecondary

private val sampleGames = listOf(
    GameCard("Slots Empire", "Hot", AccentRed),
    GameCard("Blackjack Pro", "New", AccentGreen),
    GameCard("Roulette Gold", "Hot", AccentRed),
    GameCard("Poker Night", "New", AccentGreen),
    GameCard("Dice Roller", null, null),
    GameCard("Baccarat VIP", "New", AccentGreen),
    GameCard("Crash Rocket", "Hot", AccentRed),
    GameCard("Minesweeper", null, null)
)

private val categories = listOf("All", "Slots", "Live", "Card", "Dice", "Originals")

/**
 * Represents a single game card displayed in the lobby grid.
 *
 * @property name The display name of the game (e.g. "Slots Empire").
 * @property tag Optional badge tag shown on the card (e.g. "Hot", "New").
 * @property tagColor Background color of the tag badge, or null if no tag.
 */
data class GameCard(
    val name: String,
    val tag: String?,
    val tagColor: Color?
)

/**
 * Main composable for the Casino lobby screen.
 *
 * Renders a header with the casino branding, a search bar for filtering
 * games by name, category filter chips, and a two-column vertical grid
 * of game cards. Manages local state for search query and selected category.
 *
 * @param viewModel The [LobbyViewModel] injected via Hilt for lobby state management.
 */
@Composable
fun LobbyScreen(
    viewModel: LobbyViewModel = hiltViewModel()
) {
    var searchQuery by remember { mutableStateOf("") }
    var selectedCategory by remember { mutableStateOf("All") }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(BgDeep)
    ) {
        // Header
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 12.dp)
        ) {
            Text(
                text = "CASINO",
                color = AccentGreen,
                fontSize = 28.sp,
                fontWeight = FontWeight.ExtraBold,
                modifier = Modifier.padding(bottom = 2.dp)
            )
            Text(
                text = "Pick your game",
                color = TextSecondary,
                fontSize = 14.sp
            )
        }

        // Search bar (pill shape)
        OutlinedTextField(
            value = searchQuery,
            onValueChange = { searchQuery = it },
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp),
            placeholder = {
                Text("Search games...", color = TextMuted)
            },
            leadingIcon = {
                Icon(
                    imageVector = Icons.Filled.Search,
                    contentDescription = "Search",
                    tint = TextMuted,
                    modifier = Modifier.size(20.dp)
                )
            },
            shape = RoundedCornerShape(9999.dp),
            colors = OutlinedTextFieldDefaults.colors(
                focusedBorderColor = AccentGreen,
                unfocusedBorderColor = BgSurface,
                focusedContainerColor = BgSurface,
                unfocusedContainerColor = BgSurface,
                cursorColor = AccentGreen
            ),
            singleLine = true
        )

        Spacer(modifier = Modifier.height(12.dp))

        // Category chips (horizontal scroll)
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(start = 16.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            categories.forEach { category ->
                FilterChip(
                    selected = selectedCategory == category,
                    onClick = { selectedCategory = category },
                    label = {
                        Text(
                            text = category,
                            fontSize = 12.sp,
                            fontWeight = FontWeight.SemiBold
                        )
                    },
                    colors = FilterChipDefaults.filterChipColors(
                        selectedContainerColor = AccentGreen,
                        selectedLabelColor = BgDeep,
                        containerColor = BgSurface,
                        labelColor = TextSecondary
                    ),
                    shape = RoundedCornerShape(6.dp)
                )
            }
        }

        Spacer(modifier = Modifier.height(12.dp))

        // Game grid
        LazyVerticalGrid(
            columns = GridCells.Fixed(2),
            contentPadding = PaddingValues(horizontal = 16.dp, vertical = 8.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            modifier = Modifier.fillMaxSize()
        ) {
            items(sampleGames) { game ->
                GameCardView(game = game)
            }
        }
    }
}

/**
 * Renders an individual game card in the lobby grid.
 *
 * Displays the game name at the bottom-left corner and an optional
 * colored tag badge above it. The card uses a vertical gradient
 * background and is clickable for future navigation.
 *
 * @param game The [GameCard] data to render.
 */
@Composable
private fun GameCardView(game: GameCard) {
    Box(
        modifier = Modifier
            .fillMaxWidth()
            .aspectRatio(3f / 4f)
            .clip(RoundedCornerShape(8.dp))
            .background(
                Brush.verticalGradient(
                    colors = listOf(BgPrimary, BgElevated)
                )
            )
            .clickable { /* Navigate to game */ },
        contentAlignment = Alignment.BottomStart
    ) {
        // Tag badge
        if (game.tag != null && game.tagColor != null) {
            Box(
                modifier = Modifier
                    .padding(8.dp)
                    .clip(RoundedCornerShape(4.dp))
                    .background(game.tagColor)
                    .padding(horizontal = 8.dp, vertical = 2.dp),
                contentAlignment = Alignment.Center
            ) {
                Text(
                    text = game.tag,
                    color = TextPrimary,
                    fontSize = 10.sp,
                    fontWeight = FontWeight.Bold
                )
            }
        }

        // Game name
        Text(
            text = game.name,
            color = TextPrimary,
            fontSize = 14.sp,
            fontWeight = FontWeight.SemiBold,
            modifier = Modifier.padding(12.dp)
        )
    }
}
