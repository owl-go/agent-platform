package expertpackage

import (
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/icon"
	"encoding/base64"
	"fmt"
	"strings"
)

func readAvatar(builtin, name string, files map[string][]byte, used map[string]bool) (string, error) {
	if name == "" {
		if icon.IsDataURL(builtin) {
			return "", fmt.Errorf("%w: image bytes require an avatar_file", domain.ErrInvalid)
		}
		return builtin, nil
	}
	content, ok := files[name]
	if !ok || !strings.HasPrefix(name, "avatars/") || builtin != "" {
		return "", fmt.Errorf("%w: invalid avatar reference", domain.ErrInvalid)
	}
	media, err := icon.ValidateImage(content)
	if err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrInvalid, err)
	}
	used[name] = true
	return "data:" + media + ";base64," + base64.StdEncoding.EncodeToString(content), nil
}
func exportAvatar(value, name string, files map[string][]byte) (string, string, error) {
	if !icon.IsDataURL(value) {
		return value, "", nil
	}
	if err := icon.ValidateProfile(value); err != nil {
		return "", "", err
	}
	parts := strings.SplitN(value, ",", 2)
	data, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", err
	}
	files[name] = data
	return "", name, nil
}
