package domain

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

type EmbeddingConfiguration struct {
	Endpoint         string    `json:"endpoint"`
	Model            string    `json:"model"`
	Dimensions       int       `json:"dimensions"`
	APIKeyConfigured bool      `json:"api_key_configured"`
	Enabled          bool      `json:"enabled"`
	Version          int64     `json:"version"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (configuration EmbeddingConfiguration) Validate() error {
	if strings.TrimSpace(configuration.Endpoint) == "" {
		return fmt.Errorf("%w: embedding endpoint is required", ErrInvalid)
	}
	parsed, err := url.Parse(configuration.Endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("%w: embedding endpoint must be an HTTPS URL", ErrInvalid)
	}
	if strings.TrimSpace(configuration.Model) == "" || len([]rune(configuration.Model)) > 200 {
		return fmt.Errorf("%w: embedding model is required", ErrInvalid)
	}
	if configuration.Dimensions != 1536 {
		return fmt.Errorf("%w: current PostgreSQL vector index requires 1536 dimensions", ErrInvalid)
	}
	return nil
}
