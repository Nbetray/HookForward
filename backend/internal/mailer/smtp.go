package mailer

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/smtp"
	"strings"

	"hookforward/backend/internal/config"
)

type EmailSender interface {
	SendVerificationCode(ctx context.Context, purpose string, toEmail string, code string) error
}

type SMTPSender struct {
	host      string
	port      int
	auth      smtp.Auth
	fromEmail string
	fromName  string
	appName   string
}

func NewSMTPSender(cfg config.Config) *SMTPSender {
	host := strings.TrimSpace(cfg.SMTPHost)
	fromEmail := strings.TrimSpace(cfg.SMTPFromEmail)
	if host == "" || fromEmail == "" {
		return nil
	}

	var auth smtp.Auth
	if strings.TrimSpace(cfg.SMTPUsername) != "" {
		auth = smtp.PlainAuth("", cfg.SMTPUsername, cfg.SMTPPassword, host)
	}

	return &SMTPSender{
		host:      host,
		port:      cfg.SMTPPort,
		auth:      auth,
		fromEmail: fromEmail,
		fromName:  strings.TrimSpace(cfg.SMTPFromName),
		appName:   cfg.AppName,
	}
}

func (s *SMTPSender) SendVerificationCode(_ context.Context, purpose string, toEmail string, code string) error {
	subject, title, description := emailContent(purpose)
	msg := buildEmailMessage(s.fromEmail, s.fromName, toEmail, subject, title, description, code, s.appName)

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	to := []string{strings.TrimSpace(toEmail)}

	if s.port == 465 {
		return s.sendMailImplicitTLS(addr, to, msg)
	}
	return smtp.SendMail(addr, s.auth, s.fromEmail, to, msg)
}

func (s *SMTPSender) sendMailImplicitTLS(addr string, to []string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: s.host})
	if err != nil {
		return fmt.Errorf("tls dial: %w", err)
	}

	host, _, _ := net.SplitHostPort(addr)
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp new client: %w", err)
	}
	defer c.Close()

	if s.auth != nil {
		if err := c.Auth(s.auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := c.Mail(s.fromEmail); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}
	for _, addr := range to {
		if err := c.Rcpt(addr); err != nil {
			return fmt.Errorf("smtp rcpt: %w", err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	return c.Quit()
}

func emailContent(purpose string) (subject string, title string, description string) {
	switch purpose {
	case "reset":
		return "重置密码验证码", "重置密码", "你正在重置密码，验证码如下："
	default:
		return "注册验证码", "邮箱验证", "你正在注册账号，验证码如下："
	}
}

func buildEmailMessage(fromEmail string, fromName string, toEmail string, subject string, title string, description string, code string, appName string) []byte {
	html := fmt.Sprintf(`
<html>
  <body style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;color:#102a43;">
    <div style="max-width:520px;margin:0 auto;padding:24px;">
      <p style="margin:0 0 8px;font-size:12px;letter-spacing:0.12em;text-transform:uppercase;color:#486581;">%s</p>
      <h2 style="margin:0 0 16px;">%s</h2>
      <p style="margin:0 0 16px;">%s</p>
      <div style="margin:20px 0;padding:16px 20px;background:#f0f9ff;border-radius:12px;font-size:28px;font-weight:700;letter-spacing:0.18em;color:#0f766e;">%s</div>
      <p style="margin:0;color:#486581;">验证码 5 分钟内有效。</p>
    </div>
  </body>
</html>`, escapeHTML(appName), escapeHTML(title), escapeHTML(description), escapeHTML(code))

	var buf bytes.Buffer
	writeHeader(&buf, "MIME-Version", "1.0")
	writeHeader(&buf, "Content-Type", `text/html; charset="UTF-8"`)
	writeHeader(&buf, "Content-Transfer-Encoding", "base64")
	writeHeader(&buf, "From", formatAddress(fromName, fromEmail))
	writeHeader(&buf, "To", strings.TrimSpace(toEmail))
	writeHeader(&buf, "Subject", subject)
	buf.WriteString("\r\n")

	encoded := base64.StdEncoding.EncodeToString([]byte(strings.TrimSpace(html)))
	for len(encoded) > 76 {
		buf.WriteString(encoded[:76])
		buf.WriteString("\r\n")
		encoded = encoded[76:]
	}
	buf.WriteString(encoded)
	buf.WriteString("\r\n")

	return buf.Bytes()
}

func writeHeader(buf *bytes.Buffer, key string, value string) {
	buf.WriteString(key)
	buf.WriteString(": ")
	buf.WriteString(value)
	buf.WriteString("\r\n")
}

func formatAddress(name string, email string) string {
	if strings.TrimSpace(name) == "" {
		return email
	}
	return fmt.Sprintf("%s <%s>", name, email)
}

func escapeHTML(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(value)
}
