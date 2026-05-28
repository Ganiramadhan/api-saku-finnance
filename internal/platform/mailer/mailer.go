package mailer

import (
	"crypto/tls"
	"fmt"
	"log"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
)

type Config struct {
	Mailer     string
	Host       string
	Port       string
	Username   string
	Password   string
	Encryption string
	FromEmail  string
	FromName   string
}

type Mailer interface {
	Send(to, subject, body string) error
}

type SMTPMailer struct{ cfg Config }

func New(cfg Config) Mailer {
	if cfg.Mailer == "" || strings.EqualFold(cfg.Mailer, "log") || cfg.Host == "" {
		log.Printf("mailer: using log mailer mailer=%q host_set=%t from=%q", cfg.Mailer, cfg.Host != "", cfg.FromEmail)
		return LogMailer{}
	}
	cfg.Encryption = normalizeEncryption(cfg.Encryption, cfg.Port)
	log.Printf(
		"mailer: smtp enabled host=%s port=%s encryption=%s username_set=%t from=%q",
		cfg.Host,
		cfg.Port,
		cfg.Encryption,
		strings.TrimSpace(cfg.Username) != "",
		cfg.FromEmail,
	)
	return &SMTPMailer{cfg: cfg}
}

func (m *SMTPMailer) Send(to, subject, body string) error {
	if strings.TrimSpace(to) == "" {
		return fmt.Errorf("mail: recipient is empty")
	}
	fromEmail := strings.TrimSpace(m.cfg.FromEmail)
	if fromEmail == "" {
		fromEmail = strings.TrimSpace(m.cfg.Username)
	}
	if fromEmail == "" {
		return fmt.Errorf("mail: sender is empty")
	}

	addr := net.JoinHostPort(m.cfg.Host, m.cfg.Port)
	auth := smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
	from := mail.Address{Name: m.cfg.FromName, Address: fromEmail}
	contentType := `text/plain; charset="utf-8"`
	if strings.Contains(strings.ToLower(body), "<html") {
		contentType = `text/html; charset="utf-8"`
	}
	headers := map[string]string{
		"From":         from.String(),
		"To":           to,
		"Subject":      mime.QEncoding.Encode("utf-8", subject),
		"MIME-Version": "1.0",
		"Content-Type": contentType,
	}
	var msg strings.Builder
	for key, value := range headers {
		msg.WriteString(key)
		msg.WriteString(": ")
		msg.WriteString(value)
		msg.WriteString("\r\n")
	}
	msg.WriteString("\r\n")
	msg.WriteString(body)

	if strings.EqualFold(m.cfg.Encryption, "ssl") {
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12})
		if err != nil {
			return m.wrapErr("connect implicit tls", err)
		}
		client, err := smtp.NewClient(conn, m.cfg.Host)
		if err != nil {
			return m.wrapErr("create smtp client", err)
		}
		defer client.Close()
		if m.cfg.Username != "" {
			if err := client.Auth(auth); err != nil {
				return m.wrapErr("authenticate", err)
			}
		}
		if err := client.Mail(fromEmail); err != nil {
			return m.wrapErr("set sender", err)
		}
		if err := client.Rcpt(to); err != nil {
			return m.wrapErr("set recipient", err)
		}
		w, err := client.Data()
		if err != nil {
			return m.wrapErr("open data writer", err)
		}
		if _, err := w.Write([]byte(msg.String())); err != nil {
			return m.wrapErr("write message", err)
		}
		if err := w.Close(); err != nil {
			return m.wrapErr("close data writer", err)
		}
		if err := client.Quit(); err != nil {
			return m.wrapErr("quit smtp", err)
		}
		log.Printf("mailer: smtp sent to=%q subject=%q host=%s port=%s encryption=%s", to, subject, m.cfg.Host, m.cfg.Port, m.cfg.Encryption)
		return nil
	}

	if err := smtp.SendMail(addr, auth, fromEmail, []string{to}, []byte(msg.String())); err != nil {
		return m.wrapErr("send mail", err)
	}
	log.Printf("mailer: smtp sent to=%q subject=%q host=%s port=%s encryption=%s", to, subject, m.cfg.Host, m.cfg.Port, m.cfg.Encryption)
	return nil
}

type LogMailer struct{}

func (LogMailer) Send(_, _, _ string) error { return nil }

func normalizeEncryption(value, port string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "ssl", "smtps", "implicit_tls", "implicit-tls", "true":
		return "ssl"
	case "tls", "starttls", "start_tls", "start-tls":
		return "tls"
	case "", "none", "false", "off":
		if strings.TrimSpace(port) == "465" {
			return "ssl"
		}
		return "tls"
	default:
		if strings.TrimSpace(port) == "465" {
			return "ssl"
		}
		return normalized
	}
}

func (m *SMTPMailer) wrapErr(step string, err error) error {
	return fmt.Errorf(
		"mail smtp %s failed host=%s port=%s encryption=%s username_set=%t from=%q: %w",
		step,
		m.cfg.Host,
		m.cfg.Port,
		m.cfg.Encryption,
		strings.TrimSpace(m.cfg.Username) != "",
		m.cfg.FromEmail,
		err,
	)
}
