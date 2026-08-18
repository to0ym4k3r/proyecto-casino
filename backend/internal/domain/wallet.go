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

// WithdrawalStatus represents the lifecycle state of a crypto withdrawal.
type WithdrawalStatus string

const (
	WithdrawalStatusPending        WithdrawalStatus = "pending"
	WithdrawalStatusPendingApproval WithdrawalStatus = "pending_approval"
	WithdrawalStatusApproved       WithdrawalStatus = "approved"
	WithdrawalStatusRejected       WithdrawalStatus = "rejected"
	WithdrawalStatusCompleted      WithdrawalStatus = "completed"
)

// Withdrawal represents a crypto withdrawal request.
// Amount is in integer cents (e.g. 10000 = $100.00).
type Withdrawal struct {
	ID         uuid.UUID        `json:"id"`
	UserID     uuid.UUID        `json:"user_id"`
	Amount     int64            `json:"amount"`
	Currency   string           `json:"currency"`
	ToAddress  string           `json:"to_address"`
	TxHash     string           `json:"tx_hash,omitempty"`
	Status     WithdrawalStatus `json:"status"`
	CreatedAt  time.Time        `json:"created_at"`
	ResolvedAt *time.Time       `json:"resolved_at,omitempty"`
}

// SolanaTxType represents the type of on-chain Solana transaction.
type SolanaTxType string

const (
	SolanaTxTypeDeposit  SolanaTxType = "deposit"
	SolanaTxTypeWithdraw SolanaTxType = "withdraw"
)

// SolanaTx represents an enriched on-chain Solana transaction record.
type SolanaTx struct {
	ID            uuid.UUID   `json:"id"`
	WalletID      uuid.UUID   `json:"wallet_id"`
	TxHash        string      `json:"tx_hash"`
	Type          SolanaTxType `json:"type"`
	Amount        int64       `json:"amount"`
	FromAddress   string      `json:"from_address"`
	ToAddress     string      `json:"to_address"`
	Confirmations int         `json:"confirmations"`
	NetworkFee    int64       `json:"network_fee"`
	UsdValue      int64       `json:"usd_value_at_time"`
	CreatedAt     time.Time   `json:"created_at"`
}

// TxHistoryFilter defines filters for transaction history queries.
type TxHistoryFilter struct {
	Type   string     // filter by SolanaTxType
	After  *time.Time // only transactions after this time
	Before *time.Time // only transactions before this time
	Page   int        // 1-indexed page number
	Limit  int        // max results per page (default 50)
}

// TxHistoryResult holds a paginated transaction history response.
type TxHistoryResult struct {
	Transactions []SolanaTx `json:"transactions"`
	Total        int        `json:"total"`
	Page         int        `json:"page"`
	Limit        int        `json:"limit"`
}
