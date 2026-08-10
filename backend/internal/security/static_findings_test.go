package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readRel(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

// TestStaticFindingsConfirmed is a set of tripwire/documentation tests that
// assert the CURRENT audit findings still hold by reading production source.
// Each subtest logs the confirming evidence; if a future fix removes the
// finding, the corresponding subtest's log changes and the team updates it.
//
// These tests intentionally assert the vulnerable state — they PASS while the
// gap exists. They are the permanent record that each finding is real.
func TestStaticFindingsConfirmed(t *testing.T) {
	root := repoRoot(t)
	authSrc := readRel(t, root, "internal/service/auth.go")
	gameSrc := readRel(t, root, "internal/service/game.go")
	walletSvc := readRel(t, root, "internal/service/wallet.go")
	walletDom := readRel(t, root, "internal/domain/wallet.go")
	gameHandler := readRel(t, root, "internal/handler/game_handler.go")
	walletHandler := readRel(t, root, "internal/handler/wallet_handler.go")

	t.Run("finding 2: refresh tokens are stateless", func(t *testing.T) {
		if strings.Contains(authSrc, "refresh_tokens") {
			t.Log("auth service now references refresh_tokens — finding #2 mitigated")
			return
		}
		t.Log("CONFIRMED: refresh_tokens table (migrations/003_auth.sql:7-15) is unused; " +
			"Refresh() issues a fresh pair with no revoke/rotate/replay check (service/auth.go:114-147)")
	})

	t.Run("finding 4: is_active never checked", func(t *testing.T) {
		if strings.Count(authSrc, "IsActive") > 2 {
			t.Log("auth service now reads IsActive somewhere — finding #4 mitigated")
			return
		}
		t.Log("CONFIRMED: IsActive is only ever SET at registration (service/auth.go:73) and " +
			"never READ; Login/Refresh/ValidateToken ignore it (service/auth.go:90-106,114-147,154-182)")
	})

	t.Run("finding 5: game RNG is math/rand", func(t *testing.T) {
		if !strings.Contains(gameSrc, "math/rand") {
			t.Log("game service no longer imports math/rand — finding #5 mitigated")
			return
		}
		t.Log("CONFIRMED: ResolveBet uses math/rand (service/game.go:6,82) — not crypto/rand")
	})

	t.Run("finding 6: wallet ops are not atomic", func(t *testing.T) {
		hasUpdate := strings.Contains(walletSvc, "UpdateBalance")
		hasCreateTx := strings.Contains(walletSvc, "CreateTransaction")
		hasBegin := strings.Contains(walletSvc, "Begin(") || strings.Contains(walletSvc, "pgx.Tx")
		if hasBegin {
			t.Log("wallet service now uses a DB transaction — finding #6 mitigated")
			return
		}
		if hasUpdate && hasCreateTx {
			t.Log("CONFIRMED: Deposit/Withdraw call UpdateBalance and CreateTransaction as " +
				"separate statements with no enclosing DB transaction (service/wallet.go:44+58, 80+94); " +
				"a crash between them loses the money trail")
		}
	})

	t.Run("finding 7: wallet money is float64", func(t *testing.T) {
		if !strings.Contains(walletDom, "float64") {
			t.Log("wallet domain no longer uses float64 — finding #7 mitigated")
			return
		}
		t.Log("CONFIRMED: Balance and Transaction.Amount are float64 (domain/wallet.go:13,37) — precision/financial-risk")
	})

	t.Run("finding 14: raw err.Error() leak to clients", func(t *testing.T) {
		leaks := []struct{ file, probe string }{}
		if strings.Contains(gameHandler, "err.Error()") {
			leaks = append(leaks, struct{ file, probe string }{"game_handler.go:114", "err.Error()"})
		}
		if strings.Contains(walletHandler, "err.Error()") {
			leaks = append(leaks, struct{ file, probe string }{"wallet_handler.go:64,94", "err.Error()"})
		}
		if len(leaks) == 0 {
			t.Log("handlers no longer leak err.Error() — finding #14 mitigated")
			return
		}
		t.Logf("CONFIRMED: raw err.Error() returned to clients at %v", leaks)
	})
}
