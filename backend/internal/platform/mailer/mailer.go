// Package mailer sends transactional email through SMTP.
//
// The requirements only demand one message: the password-reset link. Account
// verification uses the same channel. When SMTP is not configured the log
// sender prints the message instead, so local development never blocks.
package mailer

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"
)

type Message struct {
	To      []string
	Subject string
	Text    string
}

type Mailer interface {
	Send(ctx context.Context, msg Message) error
}

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

func New(cfg SMTPConfig, log *slog.Logger) Mailer {
	if strings.TrimSpace(cfg.Host) == "" {
		return &logMailer{log: log}
	}
	return &smtpMailer{cfg: cfg}
}

type smtpMailer struct {
	cfg SMTPConfig
}

func (m *smtpMailer) Send(ctx context.Context, msg Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	addr := net.JoinHostPort(m.cfg.Host, fmt.Sprintf("%d", m.cfg.Port))

	var auth smtp.Auth
	if m.cfg.Username != "" {
		host := m.cfg.Host
		auth = smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, host)
	}

	return smtp.SendMail(addr, auth, envelopeFrom(m.cfg.From), msg.To, buildMessage(m.cfg.From, msg))
}

func envelopeFrom(from string) string {
	if i := strings.LastIndex(from, "<"); i >= 0 {
		if j := strings.LastIndex(from, ">"); j > i {
			return strings.TrimSpace(from[i+1 : j])
		}
	}
	return strings.TrimSpace(from)
}

func buildMessage(from string, msg Message) []byte {
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + strings.Join(msg.To, ", ") + "\r\n")
	b.WriteString("Subject: " + msg.Subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(msg.Text)
	b.WriteString("\r\n")
	return []byte(b.String())
}

type logMailer struct {
	log *slog.Logger
}

func (m *logMailer) Send(_ context.Context, msg Message) error {
	m.log.Info("mail not sent (SMTP not configured)",
		"to", msg.To,
		"subject", msg.Subject,
		"body", msg.Text,
	)
	return nil
}
