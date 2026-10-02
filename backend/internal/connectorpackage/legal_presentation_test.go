package connectorpackage

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"strings"
	"testing"

	"agent-platform/backend/internal/icon"
)

func TestLegalConnectorBrandIcons(t *testing.T) {
	for _, test := range []struct {
		source string
		asset  []byte
		size   int
	}{{"pkulaw", pkulawIcon, 32}, {"mindbye", mindbyeIcon, 64}} {
		t.Run(test.source, func(t *testing.T) {
			value := DisplayIcon(test.source)
			if err := icon.Validate(value); err != nil {
				t.Fatal(err)
			}
			encoded, ok := strings.CutPrefix(value, "data:image/png;base64,")
			if !ok {
				t.Fatal("missing supported brand image")
			}
			body, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil || !bytes.Equal(body, test.asset) {
				t.Fatal("icon differs from reviewed brand asset")
			}
			config, err := png.DecodeConfig(bytes.NewReader(body))
			if err != nil || config.Width != test.size || config.Height != test.size {
				t.Fatalf("brand image: %v, %v", config, err)
			}
		})
	}
}
