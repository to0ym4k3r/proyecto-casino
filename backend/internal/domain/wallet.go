package domain

import (
	"time"

	"github.com/google/uuid"
)

// Wallet represents a user's financial balance.
// Balance is stored in integer cents (e.g. 15000 = $150.00).
type Wallet struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Balance   int64     `json:"balance"`
	Currency  string    `json:"currency"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
// Amount and Balance are in integer cents.
type Transaction struct {
	ID        uuid.UUID       `json:"id"`
	WalletID  uuid.UUID       `json:"wallet_id"`
	Type      TransactionType `json:"type"`
	Amount    int64           `json:"amount"`
	Balance   int64           `json:"balance_after"`
	Reference string          `json:"reference"`
	CreatedAt time.Time       `json:"created_at"`
}

// DepositRequest is the payload for depositing funds.
// Amount is in integer cents (e.g. 10000 = $100.00).
type DepositRequest struct {
	Amount int64 `json:"amount" validate:"required,gt=0,lte=10000000"`
}

// WithdrawRequest is the payload for withdrawing funds.
// Amount is in integer cents.
type WithdrawRequest struct {
	Amount int64 `json:"amount" validate:"required,gt=0"`
}
