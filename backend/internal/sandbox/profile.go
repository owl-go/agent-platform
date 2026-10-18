package sandbox

import (
	"fmt"
	"regexp"
)

const LocalProfile = "local"

var registryImage = regexp.MustCompile(`^[^\s@]+@sha256:[a-f0-9]{64}$`)
var localImage = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

// ValidateExecutionProfile never implicitly downgrades a production sandbox.
func ValidateExecutionProfile(profile, runtime string) error {
	switch profile {
	case "", "production":
		if runtime != "runsc" {
			return fmt.Errorf("production sandbox runtime must be runsc")
		}
	case LocalProfile:
		if runtime != "runc" {
			return fmt.Errorf("local sandbox runtime must be runc")
		}
	default:
		return fmt.Errorf("sandbox profile must be production or local")
	}
	return nil
}

func ValidExecutionImage(profile, image string) bool {
	return registryImage.MatchString(image) || profile == LocalProfile && localImage.MatchString(image)
}
