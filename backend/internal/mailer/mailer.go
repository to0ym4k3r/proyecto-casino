package mailer

import (
	"context"
	"fmt"
)

// Mailer delivers outbound email messages to players. The console driver is
// the development default; production selects the SMTP driver via
// MAILER_DRIVER=smtp.
type Mailer interface {
	// SendVerificationEmail delivers the single-use verification link for the
	// player's email address (or logs it under the console driver).
	SendVerificationEmail(ctx context.Context, to, token string) error
}

// verifyLink builds the public verification URL for a raw token.
func verifyLink(baseURL, token string) string {
	return fmt.Sprintf("%s/auth/verify-email?token=%s", baseURL, token)
}
