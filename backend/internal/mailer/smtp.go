package mailer

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"

	"casino-backend/internal/config"
)

// SMTPMailer sends verification emails over SMTP with optional STARTTLS
// (SMTP_TLS=true, the default) and SMTP AUTH when a user is configured.
//
// Outbound connections only go through smtp.Dial: the security suite rejects
// raw outbound dial calls in production source, and TLS is always configured
// with a ServerName (never with certificate verification disabled).
type SMTPMailer struct {
	cfg config.MailerConfig
}

// NewSMTPMailer creates an SMTP mailer from the mailer configuration.
func NewSMTPMailer(cfg config.MailerConfig) *SMTPMailer {
	return &SMTPMailer{cfg: cfg}
}

// SendVerificationEmail sends the single-use verification link to the player's
// inbox. The context is honored before any network I/O begins.
func (m SMTPMailer) SendVerificationEmail(ctx context.Context, to, token string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	addr := net.JoinHostPort(m.cfg.SMTPHost, m.cfg.SMTPPort)

	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("dial smtp server: %w", err)
	}
	defer client.Close()

	if err := client.Hello(m.cfg.SMTPHost); err != nil {
		return fmt.Errorf("smtp greeting: %w", err)
	}

	if m.cfg.SMTPTLS {
		if err := client.StartTLS(&tls.Config{ServerName: m.cfg.SMTPHost}); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}

	if m.cfg.SMTPUser != "" {
		auth := smtp.PlainAuth("", m.cfg.SMTPUser, m.cfg.SMTPPassword, m.cfg.SMTPHost)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := client.Mail(m.cfg.SMTPFrom); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}

	body := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: Verify your email\r\n"+
			"MIME-Version: 1.0\r\nContent-Type: text/plain; charset=\"utf-8\"\r\n\r\n"+
			"Verify your email address by opening this link:\r\n%s\r\n",
		m.cfg.SMTPFrom, to, verifyLink(m.cfg.VerifyBaseURL, token),
	)
	if _, err := w.Write([]byte(body)); err != nil {
		return fmt.Errorf("write smtp message: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close smtp message: %w", err)
	}

	return client.Quit()
}

var _ Mailer = SMTPMailer{}
