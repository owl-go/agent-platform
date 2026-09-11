package icon

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
)

const MaxDataURLBytes = 384 * 1024

var (
	slugPattern    = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
	dataURLPattern = regexp.MustCompile(`^data:image/(png|jpeg|webp|gif);base64,([A-Za-z0-9+/]*={0,2})$`)
)

func Validate(value string) error {
	if value == "" || slugPattern.MatchString(value) {
		return nil
	}
	match := dataURLPattern.FindStringSubmatch(value)
	if match == nil {
		return fmt.Errorf("icon must be a built-in name or a supported image")
	}
	decoded, err := base64.StdEncoding.DecodeString(match[2])
	if err != nil || len(decoded) == 0 || len(decoded) > MaxDataURLBytes {
		return fmt.Errorf("icon image must be at most %d bytes", MaxDataURLBytes)
	}
	return nil
}

func IsDataURL(value string) bool { return strings.HasPrefix(value, "data:image/") }
