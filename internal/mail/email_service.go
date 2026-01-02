// Package mail provides email sending services and related utilities.
package mail

import (
	"fluxqueue/internal/config"

	"github.com/rs/zerolog/log"
	"gopkg.in/gomail.v2"
)

type EmailService struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	UseTLS   bool
}

func NewEmailService(cfg config.EmailConfig) *EmailService {
	return &EmailService{
		Host:     cfg.Host,
		Port:     cfg.Port,
		Username: cfg.Username,
		Password: cfg.Password,
		From:     cfg.From,
		UseTLS:   cfg.UseTLS,
	}
}

func (e *EmailService) SendEmail(to, subject, body string) error {
	m := gomail.NewMessage()

	m.SetHeader("From", e.From)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	// dial the SMTP server
	d := gomail.NewDialer(e.Host, e.Port, e.Username, e.Password)
	d.TLSConfig = nil

	if err := d.DialAndSend(m); err != nil {
		return err
	}
	log.Info().Msg("Email sent successfully")
	return nil
}
