package mailer

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
	"time"

	"github.com/flaboy/aira-core/pkg/config"

	"github.com/knadh/smtppool/v2"
	"github.com/resend/resend-go/v3"
)

type Mailer struct {
	Host         string
	Port         int
	Username     string
	Password     string
	TLS          string
	MailFrom     string
	pool         *smtppool.Pool
	ResendAPIKey string
	UseResend    bool
	resendClient *resend.Client
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
		Auth:            auth,             // Will be nil if username is empty
		IdleTimeout:     time.Second * 30, // 增加空闲超时，保持连接更久
		PoolWaitTimeout: time.Second * 5,  // 减少等待超时，更快发现连接问题
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
	resendAPIKey := config.Config.SendMail.ResendAPIKey
	useResend := resendAPIKey != ""

	slog.Info("[Mailer] ========== InitSMTP START ==========",
		"resend_api_key_configured", resendAPIKey != "",
		"resend_api_key_length", len(resendAPIKey),
		"resend_api_key_prefix", func() string {
			if len(resendAPIKey) > 8 {
				return resendAPIKey[:8] + "..."
			}
			return "empty"
		}(),
		"use_resend", useResend,
		"smtp_host", config.Config.SendMail.Host,
		"smtp_port", config.Config.SendMail.Port)

	SMTPSender = &Mailer{
		Host:         config.Config.SendMail.Host,
		Port:         config.Config.SendMail.Port,
		Username:     config.Config.SendMail.Username,
		Password:     config.Config.SendMail.Password,
		TLS:          config.Config.SendMail.TLS,
		MailFrom:     config.Config.SendMail.From,
		ResendAPIKey: resendAPIKey,
		UseResend:    useResend,
	}

	if useResend {
		SMTPSender.resendClient = resend.NewClient(resendAPIKey)
		slog.Info("[Mailer] ========== Initializing Resend API mailer ==========",
			"from", SMTPSender.MailFrom,
			"api_key_set", SMTPSender.ResendAPIKey != "",
			"resend_client_initialized", SMTPSender.resendClient != nil)
	} else {
		SMTPSender.initPool()
		slog.Info("[Mailer] ========== Initializing SMTP mailer ==========",
			"host", config.Config.SendMail.Host,
			"port", config.Config.SendMail.Port,
			"tls", config.Config.SendMail.TLS,
			"username", config.Config.SendMail.Username,
			"reason", "RESEND_API_KEY not configured")
	}
}

func (m *Mailer) SendMail(ctx context.Context, to, subject, body string) error {
	// 如果配置了 Resend，使用 Resend API
	if m.UseResend && m.resendClient != nil {
		slog.Info("[Mailer] Routing to Resend API",
			"use_resend", m.UseResend,
			"resend_client_exists", m.resendClient != nil)
		return m.sendViaResend(ctx, to, subject, body)
	}

	// 否则使用 SMTP（原有逻辑）
	reason := "unknown"
	if !m.UseResend {
		reason = "UseResend is false"
	} else if m.resendClient == nil {
		reason = "resendClient is nil"
	}
	slog.Info("[Mailer] Routing to SMTP",
		"use_resend", m.UseResend,
		"resend_client_exists", m.resendClient != nil,
		"reason", reason)
	return m.sendViaSMTP(ctx, to, subject, body)
}

// sendViaResend 通过 Resend API 发送邮件
func (m *Mailer) sendViaResend(ctx context.Context, to, subject, body string) error {
	startTime := time.Now()
	slog.Info("[Resend] ========== SendMail START ==========",
		"to", to,
		"subject", subject,
		"timestamp", startTime.Format("15:04:05.000000"))

	params := &resend.SendEmailRequest{
		From:    m.MailFrom,
		To:      []string{to},
		Subject: subject,
		Html:    body,
	}

	slog.Info("[Resend] Email request prepared",
		"to", to,
		"from", m.MailFrom,
		"subject", subject,
		"html_length", len(body))

	sendStartTime := time.Now()
	slog.Info("[Resend] ========== Calling Resend API ==========",
		"timestamp", sendStartTime.Format("15:04:05.000000"))

	sent, err := m.resendClient.Emails.Send(params)
	sendEndTime := time.Now()

	totalDuration := sendEndTime.Sub(startTime)
	sendDuration := sendEndTime.Sub(sendStartTime)

	if err != nil {
		slog.Error("[Resend] ========== SendMail FAILED ==========",
			"to", to,
			"error", err,
			"error_type", fmt.Sprintf("%T", err),
			"error_string", err.Error(),
			"total_duration_ms", totalDuration.Milliseconds(),
			"send_duration_ms", sendDuration.Milliseconds(),
			"total_duration_seconds", totalDuration.Seconds(),
			"end_timestamp", sendEndTime.Format("15:04:05.000000"))
		return err
	}

	slog.Info("[Resend] ========== SendMail SUCCESS ==========",
		"to", to,
		"from", m.MailFrom,
		"subject", subject,
		"email_id", sent.Id,
		"total_duration_ms", totalDuration.Milliseconds(),
		"send_duration_ms", sendDuration.Milliseconds(),
		"total_duration_seconds", totalDuration.Seconds(),
		"start_timestamp", startTime.Format("15:04:05.000000"),
		"end_timestamp", sendEndTime.Format("15:04:05.000000"))

	return nil
}

// sendViaSMTP 通过 SMTP 发送邮件（原有逻辑）
func (m *Mailer) sendViaSMTP(ctx context.Context, to, subject, body string) error {
	startTime := time.Now()
	slog.Info("[SMTP] ========== SendMail START ==========",
		"to", to,
		"subject", subject,
		"timestamp", startTime.Format("15:04:05.000000"))

	sender := m.pool
	mailfrom := m.MailFrom
	slog.Info("[SMTP] Using default pool",
		"pool_exists", sender != nil,
		"mailfrom", mailfrom)

	emailPrepTime := time.Now()
	slog.Info("[SMTP] Email preparation completed",
		"duration_ms", emailPrepTime.Sub(startTime).Milliseconds(),
		"prep_timestamp", emailPrepTime.Format("15:04:05.000000"))

	e := smtppool.Email{
		To:      []string{to},
		From:    mailfrom,
		Subject: subject,
		HTML:    []byte(body),
	}

	// 尝试获取邮件原始字节，用于诊断
	emailBytes, bytesErr := e.Bytes()
	if bytesErr != nil {
		slog.Warn("[SMTP] Failed to get email bytes for logging", "error", bytesErr)
	} else {
		emailPreview := ""
		if len(emailBytes) > 500 {
			emailPreview = string(emailBytes[:500]) + "..."
		} else {
			emailPreview = string(emailBytes)
		}
		slog.Info("[SMTP] Email bytes generated",
			"email_bytes_length", len(emailBytes),
			"email_bytes_preview", emailPreview)
	}

	slog.Info("[SMTP] Email object created",
		"to", to,
		"from", mailfrom,
		"subject", subject,
		"html_length", len(body),
		"to_count", len(e.To),
		"to_list", e.To)

	sendStartTime := time.Now()
	slog.Info("[SMTP] ========== Calling pool.Send ==========",
		"timestamp", sendStartTime.Format("15:04:05.000000"),
		"pool_config", map[string]interface{}{
			"max_conns":             10,
			"idle_timeout_sec":      30,
			"pool_wait_timeout_sec": 5,
		})

	// 记录发送前的连接池状态（如果可能）
	slog.Info("[SMTP] Pre-send state",
		"pool_exists", sender != nil)

	err := sender.Send(e)

	sendEndTime := time.Now()
	totalDuration := sendEndTime.Sub(startTime)
	sendDuration := sendEndTime.Sub(sendStartTime)

	if err != nil {
		slog.Error("[SMTP] ========== SendMail FAILED ==========",
			"to", to,
			"error", err,
			"error_type", fmt.Sprintf("%T", err),
			"error_string", err.Error(),
			"total_duration_ms", totalDuration.Milliseconds(),
			"send_duration_ms", sendDuration.Milliseconds(),
			"total_duration_seconds", totalDuration.Seconds(),
			"end_timestamp", sendEndTime.Format("15:04:05.000000"))
		return err
	}

	slog.Info("[SMTP] ========== SendMail SUCCESS ==========",
		"to", to,
		"from", mailfrom,
		"subject", subject,
		"total_duration_ms", totalDuration.Milliseconds(),
		"send_duration_ms", sendDuration.Milliseconds(),
		"total_duration_seconds", totalDuration.Seconds(),
		"start_timestamp", startTime.Format("15:04:05.000000"),
		"end_timestamp", sendEndTime.Format("15:04:05.000000"))

	// 重要：记录成功但提醒检查
	slog.Warn("[SMTP] IMPORTANT: pool.Send returned nil error, but email may not be delivered",
		"check_1", "Verify email in recipient's inbox/spam folder",
		"check_2", "Check SMTP provider dashboard for delivery status",
		"check_3", "Verify sender email/domain is verified in SMTP provider",
		"check_4", "Check SMTP provider logs for any issues")

	return err
}
