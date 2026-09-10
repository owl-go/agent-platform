package promptoptimizer

import "testing"

func TestSanitizePromptKeepsRequestedLanguageAndDropsCommentary(t *testing.T) {
	input := "- 中文版（更通顺）：**一个中年男人下班后坐在车里。**\n- 英文版（用于画面描述）：**A middle-aged man sits in his car after work.**\n\n如果你是要做一段更有画面感的文案，我可以继续扩写。"
	got := sanitizePrompt(input, "一个男人下班后坐在车里", "en-US")
	if want := "一个中年男人下班后坐在车里。"; got != want {
		t.Fatalf("sanitizePrompt() = %q, want %q", got, want)
	}
}

func TestSanitizePromptRemovesMarkdownAndPromptHeading(t *testing.T) {
	input := "```text\nFinal prompt: **A cinematic cat in warm light.**\n```"
	if got, want := sanitizePrompt(input, "a cat", "en-US"), "A cinematic cat in warm light."; got != want {
		t.Fatalf("sanitizePrompt() = %q, want %q", got, want)
	}
}

func TestSanitizePromptSelectsEnglishVariant(t *testing.T) {
	input := "中文版本：一只猫在窗边。\nEnglish version: A cat sits by the window."
	if got, want := sanitizePrompt(input, "A cat by the window", "zh-CN"), "A cat sits by the window."; got != want {
		t.Fatalf("sanitizePrompt() = %q, want %q", got, want)
	}
}

func TestSanitizePromptDropsStandaloneVariantHeadingAndCapitalizedFollowUp(t *testing.T) {
	input := "English version\nA cat sits by the window.\nIf you need another style, I can revise it."
	if got, want := sanitizePrompt(input, "a cat", "zh-CN"), "A cat sits by the window."; got != want {
		t.Fatalf("sanitizePrompt() = %q, want %q", got, want)
	}
}
