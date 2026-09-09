package workspace

import (
	"testing"

	aicreationdomain "agent-platform/backend/internal/biz/aicreation/domain"
)

func TestPlatformImageOptionsChargeFiftyCreditsPerImage(t *testing.T) {
	_, sizes, qualities, _, _, rates := platformImageOptions("https://api.openai.com/v1")
	for _, size := range sizes {
		for _, quality := range qualities {
			key := aicreationdomain.RateKey(size, quality)
			if got := rates[key]; got != 5_000 {
				t.Fatalf("rate[%q, %q] = %d hundredths, want 5000", size, quality, got)
			}
		}
	}
}

func TestPlatformImageOptionsRestrictBailianNativeOutput(t *testing.T) {
	_, _, qualities, formats, backgrounds, _ := platformImageOptions("https://workspace.cn-beijing.maas.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation")
	if len(qualities) != 1 || qualities[0] != "auto" || len(formats) != 1 || formats[0] != "png" || len(backgrounds) != 1 || backgrounds[0] != "opaque" {
		t.Fatalf("Bailian options = qualities %v, formats %v, backgrounds %v", qualities, formats, backgrounds)
	}
}

func TestPlatformImageOptionsDoNotTreatSimilarPathAsBailian(t *testing.T) {
	_, _, qualities, formats, backgrounds, _ := platformImageOptions("https://example.com/proxy/api/v1/services/aigc/multimodal-generation/generation")
	if len(qualities) != 4 || len(formats) != 3 || len(backgrounds) != 2 {
		t.Fatalf("similar endpoint unexpectedly selected Bailian options: qualities %v, formats %v, backgrounds %v", qualities, formats, backgrounds)
	}
}
