package com.casino.app.ui.screens.wallet

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController

object WalletRoute {
    const val Balance = "wallet_balance"
    const val Deposit = "wallet_deposit"
    const val Withdraw = "wallet_withdraw"
    const val History = "wallet_history"
}

@Composable
fun WalletNavGraph(
    viewModel: WalletViewModel = hiltViewModel(),
) {
    val navController = rememberNavController()
    val uiState by viewModel.uiState.collectAsStateWithLifecycle()

    NavHost(
        navController = navController,
        startDestination = WalletRoute.Balance,
    ) {
        composable(WalletRoute.Balance) {
            BalanceOnlyScreen(
                uiState = uiState,
                onNavigateToDeposit = { navController.navigate(WalletRoute.Deposit) },
                onNavigateToWithdraw = { navController.navigate(WalletRoute.Withdraw) },
                onNavigateToHistory = { navController.navigate(WalletRoute.History) },
            )
        }
        composable(WalletRoute.Deposit) {
            DepositScreen(
                uiState = uiState,
                onGetSolDeposit = viewModel::getSolDeposit,
                onBack = { navController.popBackStack() },
            )
        }
        composable(WalletRoute.Withdraw) {
            WithdrawScreen(
                uiState = uiState,
                onAmountChange = viewModel::onWithdrawAmountChange,
                onAddressChange = viewModel::onWithdrawAddressChange,
                onWithdraw = viewModel::withdrawSol,
                onBack = { navController.popBackStack() },
            )
        }
        composable(WalletRoute.History) {
            TransactionHistoryScreen(
                uiState = uiState,
                onFilterByType = viewModel::filterByType,
                onClearFilter = viewModel::clearFilter,
                onBack = { navController.popBackStack() },
            )
        }
    }
}

@Composable
private fun BalanceOnlyScreen(
    uiState: WalletUiState,
    onNavigateToDeposit: () -> Unit,
    onNavigateToWithdraw: () -> Unit,
    onNavigateToHistory: () -> Unit,
) {
    WalletScreen(viewModel = hiltViewModel<WalletViewModel>())
}
