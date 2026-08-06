package domain

import (
	"time"

	"github.com/google/uuid"
)

// Wallet represents a user's financial balance.
type Wallet struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	Balance   float64    `json:"balance"`
	Currency  string     `json:"currency"`
	Version   int        `json:"version"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// TransactionType represents the type of wallet transaction.
type TransactionType string

const (
	TransactionDeposit  TransactionType = "deposit"
	TransactionWithdraw TransactionType = "withdraw"
	TransactionBet      TransactionType = "bet"
	TransactionWin      TransactionType = "win"
	TransactionRefund   TransactionType = "refund"
)

// Transaction represents a wallet transaction record.
type Transaction struct {
	ID        uuid.UUID         `json:"id"`
	WalletID  uuid.UUID         `json:"wallet_id"`
	Type      TransactionType   `json:"type"`
	Amount    float64           `json:"amount"`
	Balance   float64           `json:"balance_after"`
	Reference string            `json:"reference"`
	CreatedAt time.Time         `json:"created_at"`
}

// DepositRequest is the payload for depositing funds.
type DepositRequest struct {
	Amount float64 `json:"amount" validate:"required,gt=0,lte=100000"`
}

// WithdrawRequest is the payload for withdrawing funds.
type WithdrawRequest struct {
	Amount float64 `json:"amount" validate:"required,gt=0"`
}
