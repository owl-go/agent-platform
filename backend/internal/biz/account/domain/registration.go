package domain

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

const (
	RegistrationFeishu = "feishu"
	RegistrationWeChat = "wechat_official"
)

func RegistrationProvider(provider string) bool {
	return provider == RegistrationFeishu || provider == RegistrationWeChat
}
func RegistrationAlias(provider string) string { return "aw-" + provider }

// RegistrationSettings is platform governance, never a User Connector grant.
type RegistrationSettings struct {
	Provider          string
	Enabled           bool
	Ready             bool
	AppID             string
	AppSecret         string
	TenantKey         string
	OfficialAccountID string
	VerificationToken string
	EncodingAESKey    string
	Version           int64
}

func (s RegistrationSettings) Validate() error {
	if !RegistrationProvider(s.Provider) {
		return fmt.Errorf("unsupported registration method")
	}
	if len(s.AppID) > 128 || len(s.AppSecret) > 512 || len(s.VerificationToken) > 128 || len(s.OfficialAccountID) > 128 || len(s.EncodingAESKey) > 43 {
		return fmt.Errorf("registration configuration exceeds limits")
	}
	if !s.Enabled {
		return nil
	}
	if strings.TrimSpace(s.AppID) == "" || strings.TrimSpace(s.AppSecret) == "" {
		return fmt.Errorf("App ID and App Secret are required")
	}
	if s.Provider == RegistrationFeishu {
		if s.TenantKey == "" {
			return fmt.Errorf("verified enterprise identity is required")
		}
	} else {
		key, err := base64.StdEncoding.DecodeString(s.EncodingAESKey + "=")
		if err != nil || len(key) != 32 || len(s.EncodingAESKey) != 43 || s.VerificationToken == "" || s.OfficialAccountID == "" {
			return fmt.Errorf("official account ID, callback Token, and 43-character EncodingAESKey are required")
		}
	}
	return nil
}

type RegistrationIdentity struct {
	Subject     string
	Username    string
	DisplayName string
}

func NewRegistrationIdentity(provider, appID, externalID, name string) (RegistrationIdentity, error) {
	if !RegistrationProvider(provider) || appID == "" || externalID == "" || len(externalID) > 256 {
		return RegistrationIdentity{}, ErrUnauthenticated
	}
	digest := sha256.Sum256([]byte(provider + "\x00" + appID + "\x00" + externalID))
	subject := fmt.Sprintf("%x", digest[:])
	prefix := "fs_"
	if provider == RegistrationWeChat {
		prefix = "wx_"
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = prefix + subject[:12]
	}
	if len(name) > 100 {
		name = prefix + subject[:12]
	}
	return RegistrationIdentity{Subject: subject, Username: prefix + subject[:32], DisplayName: name}, nil
}

// Attempts are short-lived and bind the external proof to one browser and
// one Keycloak authorization request. Only an issued code can be exchanged.
type RegistrationAttempt struct {
	ID            string
	Provider      string
	ConfigVersion int64
	BrowserHash   string
	RedirectURI   string
	State         string
	Nonce         string
	Challenge     string
	QRURL         string
	TicketHash    string
	Identity      RegistrationIdentity
	Status        string
	CodeHash      string
	ExpiresAt     time.Time
	Version       int64
}
