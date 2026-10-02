package connectorpackage

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"image/png"
	"strings"
	"testing"
)

func TestCaoliaoDisplayIconContainsOfficialBrandImage(t *testing.T) {
	encoded, ok := strings.CutPrefix(DisplayIcon("caoliao"), "data:image/svg+xml;base64,")
	if !ok {
		t.Fatal("Caoliao must project a displayable brand image")
	}
	body, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	var svg struct {
		ViewBox string `xml:"viewBox,attr"`
		Image   struct {
			Href string `xml:"href,attr"`
		} `xml:"image"`
	}
	if err := xml.Unmarshal(body, &svg); err != nil || svg.ViewBox != "0 0 216 216" {
		t.Fatalf("brand viewport: %q, %v", svg.ViewBox, err)
	}
	image, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(svg.Image.Href, "data:image/png;base64,"))
	if err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(image))
	if err != nil || config.Width != 1200 || config.Height != 216 {
		t.Fatalf("embedded official logo: %v, %v", config, err)
	}
}
