package service

import "context"

// Mailer is the outbound email seam the auth flow depends on. The production
// implementations live in internal/mailer (ConsoleMailer for development,
// SMTPMailer for real delivery) and satisfy this interface structurally; tests
// plug in a stub.
type Mailer interface {
	// SendVerificationEmail delivers the single-use verification token for the
	// player's email address.
	SendVerificationEmail(ctx context.Context, to, token string) error
}
