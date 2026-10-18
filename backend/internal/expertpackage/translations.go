package expertpackage

import (
	"fmt"
	"slices"
	"strings"

	"agent-platform/backend/internal/biz/workspace/domain"
)

// DisplayTranslation is package display content, never executable guidance.
type DisplayTranslation struct {
	Name           string   `json:"name"`
	Introduction   string   `json:"introduction"`
	StarterPrompts []string `json:"starter_prompts"`
}

// DisplayTranslations keeps both author-supplied language versions explicit.
// Existing packages may omit it and continue using their default display fields.
type DisplayTranslations struct {
	Chinese DisplayTranslation `json:"zh-CN"`
	English DisplayTranslation `json:"en"`
}

func (translations *DisplayTranslations) validate(name, introduction string, prompts []string) error {
	if translations == nil {
		return nil
	}
	for _, item := range []DisplayTranslation{translations.Chinese, translations.English} {
		if strings.TrimSpace(item.Name) == "" || len(item.Name) > 100 || strings.TrimSpace(item.Introduction) == "" || len(item.Introduction) > 2000 {
			return fmt.Errorf("%w: translated name and introduction are required and must fit display limits", domain.ErrInvalid)
		}
		if err := domain.ValidateStarterPrompts(item.StarterPrompts); err != nil {
			return err
		}
		if len(item.StarterPrompts) != len(prompts) {
			return fmt.Errorf("%w: translated common tasks must correspond to the default tasks", domain.ErrInvalid)
		}
	}
	if translations.Chinese.Name != name || translations.Chinese.Introduction != introduction || !slices.Equal(translations.Chinese.StarterPrompts, prompts) {
		return fmt.Errorf("%w: Chinese translation must match the default display fields", domain.ErrInvalid)
	}
	return nil
}
