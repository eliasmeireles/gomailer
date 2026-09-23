package config

import (
	"fmt"
	"strconv"
)

const (
	envSMTPServer     = "SMTP_SERVER"
	envSMTPServerPort = "SMTP_SERVER_PORT"
	envSMTPServerUser = "SMTP_SERVER_USER"
	envSMTPServerPass = "SMTP_SERVER_PASS"
)

// SMTPConfig holds the SMTP connection configuration.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
}

// NewSMTPConfig loads the SMTP configuration from SMTP_SERVER, SMTP_SERVER_PORT,
// SMTP_SERVER_USER and SMTP_SERVER_PASS. All of them are required.
func NewSMTPConfig() (SMTPConfig, error) {
	values := map[string]string{}
	for _, key := range []string{envSMTPServer, envSMTPServerPort, envSMTPServerUser, envSMTPServerPass} {
		value, err := requireEnv(key)
		if err != nil {
			return SMTPConfig{}, err
		}
		values[key] = value
	}

	port, err := strconv.Atoi(values[envSMTPServerPort])
	if err != nil {
		return SMTPConfig{}, fmt.Errorf("%s must be a valid integer: %w", envSMTPServerPort, err)
	}

	return SMTPConfig{
		Host:     values[envSMTPServer],
		Port:     port,
		Username: values[envSMTPServerUser],
		Password: values[envSMTPServerPass],
	}, nil
}
