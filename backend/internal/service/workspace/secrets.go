package workspace

import (
	"encoding/json"
	"fmt"

	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
)

func (service *Service) encryptEnvironmentSecrets(owner, aadPrefix string, environment []workspacedomain.EnvironmentVariable, submitted map[string]string, existingCiphertext []byte) ([]byte, error) {
	existing := make(map[string]string)
	if len(existingCiphertext) > 0 {
		plaintext, err := service.box.Decrypt(existingCiphertext, aadPrefix+owner)
		if err != nil {
			return nil, fmt.Errorf("decrypt existing Secret environment: %w", err)
		}
		defer clear(plaintext)
		if err := json.Unmarshal(plaintext, &existing); err != nil {
			return nil, fmt.Errorf("decode existing Secret environment: %w", err)
		}
	}
	selected := make(map[string]string)
	for index := range environment {
		variable := &environment[index]
		if !variable.Secret {
			continue
		}
		if value, ok := submitted[variable.Name]; ok && value != "" {
			selected[variable.Name] = value
			variable.Configured = true
			continue
		}
		if variable.Configured {
			if value, ok := existing[variable.Name]; ok && value != "" {
				selected[variable.Name] = value
				continue
			}
		}
		return nil, fmt.Errorf("%w: Secret environment variable %q requires a value", workspacedomain.ErrInvalid, variable.Name)
	}
	plaintext, err := json.Marshal(selected)
	if err != nil {
		return nil, err
	}
	defer clear(plaintext)
	ciphertext, err := service.box.Encrypt(plaintext, aadPrefix+owner)
	if err != nil {
		return nil, err
	}
	return ciphertext, nil
}
