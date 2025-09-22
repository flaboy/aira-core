package mailer

import (
	"crypto/tls"
	"log"
	"net/smtp"
	"strings"
	"time"

	"github.com/flaboy/aira/aira-core/pkg/config"

	"github.com/knadh/smtppool"
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
	auth := smtp.PlainAuth("", username, password, host)
	// auth := smtp.CRAMMD5Auth(m.Username, m.Password)
	opts := smtppool.Opt{
		Host:            host,
		Port:            port,
		MaxConns:        10,
		Auth:            auth,
		IdleTimeout:     time.Second * 10,
		PoolWaitTimeout: time.Second * 10,
	}
	switch strings.ToUpper(encryption) {
	case "NONE":
		opts.TLSConfig = nil
	case "SSL":
		opts.SSL = true
		opts.TLSConfig = &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         m.Host,
		}
	case "TLS":
		opts.SSL = false
		opts.TLSConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}
	pool, _ := smtppool.New(opts)
	return pool
}

func InitSMTP() {
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

func (m *Mailer) SendTo(to, subject, body string) error {

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
	log.Printf("Send email to %s, subject: %s", to, subject)
	err := sender.Send(e)
	if err != nil {
		log.Printf("Failed to send email to %s: %v", to, err)
	}
	return err
}
