package mailer

import (
	"context"
	"log"
)

// ConsoleMailer is the development mailer (MAILER_DRIVER=console, the default):
// it logs the verification link instead of sending real email.
type ConsoleMailer struct {
	VerifyBaseURL string
}

// SendVerificationEmail logs the verification link to stdout.
func (m ConsoleMailer) SendVerificationEmail(_ context.Context, to, token string) error {
	log.Printf("verification email to %s: %s", to, verifyLink(m.VerifyBaseURL, token))
	return nil
}

var _ Mailer = ConsoleMailer{}
