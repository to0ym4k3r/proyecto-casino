package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"casino-backend/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// --- pgx.Tx / pgx.Row test doubles ---

// execCall records a single Exec against the stub transaction.
type execCall struct {
	sql  string
	args []interface{}
}

// recordingTx is a stub pgx.Tx that records Exec calls and the latest
// QueryRow SQL. Any other pgx.Tx method panics via the embedded nil
// interface — if a wallet op starts using an unsupported call, the test
// fails loudly instead of passing vacuously.
type recordingTx struct {
	pgx.Tx
	execs         []execCall
	lastQuerySQL  string
	queryRowValue pgx.Row
	// execTag is the command tag Exec returns; when unset (zero value) it
	// defaults to "UPDATE 1".
	execTag pgconn.CommandTag
}

func (t *recordingTx) Exec(_ context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	t.execs = append(t.execs, execCall{sql: sql, args: args})
	tag := t.execTag
	if tag.String() == "" {
		tag = pgconn.NewCommandTag("UPDATE 1")
	}
	return tag, nil
}

func (t *recordingTx) QueryRow(_ context.Context, sql string, _ ...interface{}) pgx.Row {
	t.lastQuerySQL = sql
	if t.queryRowValue == nil {
		panic("recordingTx.QueryRow called without a preset row")
	}
	return t.queryRowValue
}

// fakeWalletRow answers Scan with a fixed wallet row, mirroring the column
// order used by WalletService.LockForUpdate (id, balance, bonus_balance,
// version).
type fakeWalletRow struct {
	id           uuid.UUID
	balance      int64
	bonusBalance int64
	version      int
	scanErr      error
}

func (r *fakeWalletRow) Scan(dest ...interface{}) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	if len(dest) != 4 {
		return errWrongScanDest
	}
	*dest[0].(*uuid.UUID) = r.id
	*dest[1].(*int64) = r.balance
	*dest[2].(*int64) = r.bonusBalance
	*dest[3].(*int) = r.version
	return nil
}

// errWrongScanDest mirrors a column-count mismatch between the stub row and
// the code under test.
var errWrongScanDest = errors.New("fakeWalletRow: wrong number of Scan dests")

func newRecordingTx() *recordingTx { return &recordingTx{} }

// --- Tests: CreditBonusBalance ---

func TestCreditBonusBalanceAddsAmount(t *testing.T) {
	tx := newRecordingTx()
	svc := NewWalletService(nil)
	userID := uuid.New()

	if err := svc.CreditBonusBalance(context.Background(), tx, userID, 1500); err != nil {
		t.Fatalf("CreditBonusBalance: %v", err)
	}
	if len(tx.execs) != 1 {
		t.Fatalf("expected exactly 1 Exec, got %d", len(tx.execs))
	}
	call := tx.execs[0]
	if !strings.Contains(call.sql, "bonus_balance = bonus_balance +") {
		t.Errorf("SQL does not increment bonus_balance: %s", call.sql)
	}
	if !strings.Contains(call.sql, "WHERE user_id =") {
		t.Errorf("SQL must target the user's wallet row: %s", call.sql)
	}
	if len(call.args) != 2 || call.args[0] != userID || call.args[1] != int64(1500) {
		t.Errorf("args = %v, want [userID amount=1500]", call.args)
	}
}

func TestCreditBonusBalanceRejectsNegativeAmount(t *testing.T) {
	tx := newRecordingTx()
	svc := NewWalletService(nil)

	err := svc.CreditBonusBalance(context.Background(), tx, uuid.New(), -1)
	if err == nil {
		t.Fatal("expected error for negative credit amount")
	}
	if len(tx.execs) != 0 {
		t.Errorf("rejected credit must not execute SQL, got %d execs", len(tx.execs))
	}
}

func TestCreditBonusBalanceReturnsErrNotFoundOnZeroRows(t *testing.T) {
	tx := newRecordingTx()
	tx.execTag = pgconn.NewCommandTag("UPDATE 0")
	svc := NewWalletService(nil)

	err := svc.CreditBonusBalance(context.Background(), tx, uuid.New(), 500)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want domain.ErrNotFound when no wallet row matched", err)
	}
}

// --- Tests: DebitBonusBalance ---

func TestDebitBonusBalanceFloorsAtZero(t *testing.T) {
	tx := newRecordingTx()
	svc := NewWalletService(nil)
	userID := uuid.New()

	if err := svc.DebitBonusBalance(context.Background(), tx, userID, 2000); err != nil {
		t.Fatalf("DebitBonusBalance: %v", err)
	}
	if len(tx.execs) != 1 {
		t.Fatalf("expected exactly 1 Exec, got %d", len(tx.execs))
	}
	call := tx.execs[0]
	// GREATEST guard: a grant-specific debit must never push the shared
	// wallets.bonus_balance negative when several grants coexist.
	if !strings.Contains(call.sql, "GREATEST(bonus_balance -") {
		t.Errorf("debit SQL lacks zero-floor guard: %s", call.sql)
	}
	if len(call.args) != 2 || call.args[0] != userID || call.args[1] != int64(2000) {
		t.Errorf("args = %v, want [userID amount=2000]", call.args)
	}
}

func TestDebitBonusBalanceRejectsNegativeAmount(t *testing.T) {
	tx := newRecordingTx()
	svc := NewWalletService(nil)

	err := svc.DebitBonusBalance(context.Background(), tx, uuid.New(), -5)
	if err == nil {
		t.Fatal("expected error for negative debit amount")
	}
	if len(tx.execs) != 0 {
		t.Errorf("rejected debit must not execute SQL, got %d execs", len(tx.execs))
	}
}

func TestDebitBonusBalanceReturnsErrNotFoundOnZeroRows(t *testing.T) {
	tx := newRecordingTx()
	tx.execTag = pgconn.NewCommandTag("UPDATE 0")
	svc := NewWalletService(nil)

	err := svc.DebitBonusBalance(context.Background(), tx, uuid.New(), 500)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want domain.ErrNotFound when no wallet row matched", err)
	}
}

// --- Tests: ZeroBonusBalance ---

func TestZeroBonusBalanceSetsColumnToZero(t *testing.T) {
	tx := newRecordingTx()
	svc := NewWalletService(nil)
	userID := uuid.New()

	if err := svc.ZeroBonusBalance(context.Background(), tx, userID); err != nil {
		t.Fatalf("ZeroBonusBalance: %v", err)
	}
	if len(tx.execs) != 1 {
		t.Fatalf("expected exactly 1 Exec, got %d", len(tx.execs))
	}
	call := tx.execs[0]
	if !strings.Contains(call.sql, "bonus_balance = 0") {
		t.Errorf("SQL does not zero bonus_balance: %s", call.sql)
	}
	if len(call.args) != 1 || call.args[0] != userID {
		t.Errorf("args = %v, want [userID]", call.args)
	}
}

func TestZeroBonusBalanceReturnsErrNotFoundOnZeroRows(t *testing.T) {
	tx := newRecordingTx()
	tx.execTag = pgconn.NewCommandTag("UPDATE 0")
	svc := NewWalletService(nil)

	err := svc.ZeroBonusBalance(context.Background(), tx, uuid.New())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want domain.ErrNotFound when no wallet row matched", err)
	}
}

// --- Tests: LockForUpdate ---

func TestLockForUpdateHydratesWallet(t *testing.T) {
	walletID := uuid.New()
	userID := uuid.New()
	tx := &recordingTx{
		queryRowValue: &fakeWalletRow{id: walletID, balance: 8000, bonusBalance: 1200, version: 3},
	}
	svc := NewWalletService(nil)

	wallet, err := svc.LockForUpdate(context.Background(), tx, userID)
	if err != nil {
		t.Fatalf("LockForUpdate: %v", err)
	}
	if wallet.ID != walletID || wallet.UserID != userID {
		t.Errorf("wallet IDs = (%v, %v), want (%v, %v)", wallet.ID, wallet.UserID, walletID, userID)
	}
	if wallet.Balance != 8000 || wallet.BonusBalance != 1200 {
		t.Errorf("balances = (%d, %d), want (8000, 1200)", wallet.Balance, wallet.BonusBalance)
	}
	if wallet.Version != 3 {
		t.Errorf("Version = %d, want 3", wallet.Version)
	}
	if !strings.Contains(tx.lastQuerySQL, "FOR UPDATE") {
		t.Errorf("lock SQL missing FOR UPDATE: %s", tx.lastQuerySQL)
	}
	if strings.Contains(tx.lastQuerySQL, "*") {
		t.Errorf("lock SQL must use an explicit column list: %s", tx.lastQuerySQL)
	}
}

func TestLockForUpdatePropagatesNotFound(t *testing.T) {
	tx := &recordingTx{
		queryRowValue: &fakeWalletRow{scanErr: pgx.ErrNoRows},
	}
	svc := NewWalletService(nil)

	_, err := svc.LockForUpdate(context.Background(), tx, uuid.New())
	if err == nil {
		t.Fatal("expected error when no wallet row exists")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want domain.ErrNotFound", err)
	}
}
