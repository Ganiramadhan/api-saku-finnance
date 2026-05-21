package mailer

import (
	"crypto/tls"
	"fmt"
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
		return LogMailer{}
	}
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
			return err
		}
		client, err := smtp.NewClient(conn, m.cfg.Host)
		if err != nil {
			return err
		}
		defer client.Close()
		if m.cfg.Username != "" {
			if err := client.Auth(auth); err != nil {
				return err
			}
		}
		if err := client.Mail(fromEmail); err != nil {
			return err
		}
		if err := client.Rcpt(to); err != nil {
			return err
		}
		w, err := client.Data()
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte(msg.String())); err != nil {
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
		return client.Quit()
	}

	return smtp.SendMail(addr, auth, fromEmail, []string{to}, []byte(msg.String()))
}

type LogMailer struct{}

func (LogMailer) Send(_, _, _ string) error { return nil }
