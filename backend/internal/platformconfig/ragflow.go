package platformconfig

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// RetrievalConfig is optional, but partial configuration always fails closed.
type RetrievalConfig struct {
	Provider string        `yaml:"provider"`
	RAGFlow  RAGFlowConfig `yaml:"ragflow"`
}
type RAGFlowConfig struct {
	Endpoint       string   `yaml:"endpoint"`
	APIKey         string   `yaml:"api_key"`
	DeploymentID   string   `yaml:"deployment_id"`
	EmbeddingModel string   `yaml:"embedding_model"`
	RequestTimeout Duration `yaml:"request_timeout"`
	ParseTimeout   Duration `yaml:"parse_timeout"`
}

func (c RetrievalConfig) Validate() error {
	if c.Provider == "" {
		if c.RAGFlow != (RAGFlowConfig{}) {
			return fmt.Errorf("retrieval.provider is required when RAGFlow is configured")
		}
		return nil
	}
	if c.Provider != "ragflow" {
		return fmt.Errorf("retrieval.provider must be ragflow")
	}
	r := c.RAGFlow
	u, err := url.Parse(r.Endpoint)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"))) {
		return fmt.Errorf("retrieval.ragflow.endpoint requires HTTPS or loopback HTTP with no path")
	}
	if strings.TrimSpace(r.APIKey) == "" || strings.ContainsAny(r.APIKey, "\r\n") {
		return fmt.Errorf("retrieval.ragflow.api_key is required")
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$`).MatchString(r.DeploymentID) || strings.TrimSpace(r.EmbeddingModel) == "" {
		return fmt.Errorf("retrieval.ragflow requires deployment_id and embedding_model")
	}
	if r.RequestTimeout.Value() <= 0 || r.RequestTimeout.Value() > 2*time.Minute || r.ParseTimeout.Value() <= 0 || r.ParseTimeout.Value() > 8*time.Minute {
		return fmt.Errorf("RAGFlow request_timeout must be within 2m and parse_timeout within 8m")
	}
	return nil
}
