package mailer

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net/smtp"
	"strings"
	"time"

	"github.com/flaboy/aira-core/pkg/config"

	"github.com/knadh/smtppool/v2"
)

type Mailer struct {
	Host     string
	Port     int
	Username string
	Password string
	TLS      string
	MailFrom string
	pool     *smtppool.Pool
}

var SMTPSender *Mailer

func (m *Mailer) initPool() {
	m.pool = m.getPool(m.Username, m.Password, m.Host, m.TLS, m.Port)
}

func (m *Mailer) getPool(username, password, host, encryption string, port int) *smtppool.Pool {
	var auth smtp.Auth
	// Only use authentication if username is provided
	if username != "" {
		auth = smtp.PlainAuth("", username, password, host)
		// auth := smtp.CRAMMD5Auth(m.Username, m.Password)
		slog.Info("SMTP auth enabled", "username", username, "host", host)
	} else {
		slog.Info("SMTP auth disabled", "host", host)
	}
	opts := smtppool.Opt{
		Host:            host,
		Port:            port,
		MaxConns:        10,
		Auth:            auth, // Will be nil if username is empty
		IdleTimeout:     time.Second * 10,
		PoolWaitTimeout: time.Second * 10,
	}
	slog.Info("SMTP pool config", "host", host, "port", port, "encryption", encryption)
	switch strings.ToUpper(encryption) {
	case "NONE":
		opts.SSL = smtppool.SSLNone
		opts.TLSConfig = nil
		slog.Info("SMTP encryption: NONE - Plain text connection")
	case "SSL":
		opts.SSL = smtppool.SSLTLS
		opts.TLSConfig = &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         host,
		}
		slog.Info("SMTP encryption: SSL/TLS", "serverName", host)
	case "TLS":
		opts.SSL = smtppool.SSLSTARTTLS
		opts.TLSConfig = &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         host,
		}
		slog.Info("SMTP encryption: STARTTLS", "serverName", host)
	}
	pool, err := smtppool.New(opts)
	if err != nil {
		slog.Error("Failed to create SMTP pool", "error", err, "host", host, "port", port)
	}
	return pool
}

func InitSMTP() {
	slog.Info("Initializing SMTP", "host", config.Config.SendMail.Host, "port", config.Config.SendMail.Port, "tls", config.Config.SendMail.TLS, "username", config.Config.SendMail.Username)
	SMTPSender = &Mailer{
		Host:     config.Config.SendMail.Host,
		Port:     config.Config.SendMail.Port,
		Username: config.Config.SendMail.Username,
		Password: config.Config.SendMail.Password,
		TLS:      config.Config.SendMail.TLS,
		MailFrom: config.Config.SendMail.From,
	}
	SMTPSender.initPool()
}

func (m *Mailer) SendMail(ctx context.Context, to, subject, body string) error {
	var sender *smtppool.Pool
	var mailfrom string

	if sender == nil {
		sender = m.pool
		mailfrom = m.MailFrom
	}

	e := smtppool.Email{
		To:      []string{to},
		From:    mailfrom,
		Subject: subject,
		HTML:    []byte(body),
	}
	slog.Info("Send email", "to", to, "subject", subject)
	err := sender.Send(e)
	if err != nil {
		slog.Error("Failed to send email", "to", to, "error", err)
	}
	return err
}
