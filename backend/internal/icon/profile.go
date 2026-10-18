package icon

import (
	"bytes"
	"encoding/base64"
	"fmt"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

const ProfileMaxBytes = 2 * 1024 * 1024

// ValidateImage shares the authenticated profile upload format and pixel limits.
func ValidateImage(content []byte) (string, error) {
	if len(content) == 0 || len(content) > ProfileMaxBytes {
		return "", fmt.Errorf("profile image size is invalid")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		return "", fmt.Errorf("decode profile image: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > 4096 || config.Height > 4096 || int64(config.Width)*int64(config.Height) > 16*1024*1024 {
		return "", fmt.Errorf("profile image dimensions are invalid")
	}
	switch format {
	case "png", "jpeg", "gif", "webp":
		return "image/" + format, nil
	default:
		return "", fmt.Errorf("unsupported profile image format")
	}
}
func ValidateProfile(value string) error {
	if value == "" || slugPattern.MatchString(value) {
		return nil
	}
	match := dataURLPattern.FindStringSubmatch(value)
	if match == nil {
		return fmt.Errorf("invalid profile icon")
	}
	data, err := base64.StdEncoding.DecodeString(match[2])
	if err != nil {
		return err
	}
	media, err := ValidateImage(data)
	if err != nil {
		return err
	}
	if media != "image/"+match[1] {
		return fmt.Errorf("profile image type mismatch")
	}
	return nil
}
