package messagechannel

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

func TestFeishuMarkdownPreservesFormattingAndCodeLiterals(t *testing.T) {
	source := "# 标题\n\n**重点** *斜体* ~~删除~~\n\n- 第一项\n- 第二项\n\n> 引用\n\n| 名称 | 状态 |\n| --- | --- |\n| 测试 | 成功 |\n\n行内 `a < b && c > d`\n\n```go\nif a < b && c > d {\n  println(\"![literal](image_key)\")\n}\n```\n\n[链接](https://example.test/?a=1&b=2)"
	got := safeCardMarkdown(source)
	if got != strings.ReplaceAll(source, "?a=1&b=2", "?a=1&amp;b=2") {
		t.Fatal("Markdown or code literals were rewritten", got)
	}
	var rendered bytes.Buffer
	markdown := goldmark.New(goldmark.WithExtensions(extension.GFM), goldmark.WithRendererOptions(html.WithUnsafe()))
	if err := markdown.Convert([]byte(got), &rendered); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"<h1>标题</h1>", "<strong>重点</strong>", "<em>斜体</em>", "<del>删除</del>", "<ul>", "<blockquote>", "<table>", `class="language-go"`, "<code>a &lt; b &amp;&amp; c &gt; d</code>"} {
		if !strings.Contains(rendered.String(), expected) {
			t.Fatal("format missing", expected, rendered.String())
		}
	}
}

func TestFeishuMarkdownMakesHTMLAndImagesInert(t *testing.T) {
	t.Parallel()
	for _, source := range []string{
		"<at id=all></at> <person id=someone></person> <img src=secret>",
		"&#60;at id=all&#62; &lt;at id=all&gt;",
		"![image](img_v2_secret) ![ref][image]\n\n[image]: img_v2_secret",
		"> ![image](img_v2_secret)\n\n<AT id=all></AT>",
		"`unclosed <at id=all></at> ![image](img_v2_secret)",
	} {
		var rendered bytes.Buffer
		markdown := goldmark.New(goldmark.WithRendererOptions(html.WithUnsafe()))
		if err := markdown.Convert([]byte(safeCardMarkdown(source)), &rendered); err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"<at ", "<AT ", "<person ", "<img "} {
			if strings.Contains(rendered.String(), forbidden) {
				t.Fatal("active provider content", rendered.String())
			}
		}
	}
}

func TestFeishuMarkdownPreservesNestedAndUnfinishedCode(t *testing.T) {
	t.Parallel()
	for _, source := range []string{
		"``literal ` and <tag> & ![image](key)``",
		"> ```html\n> <at id=all></at>\n> ```",
		"~~~html\n<at id=all></at> & ![image](key)\n~~~",
		"```go\na < b && c > d\n", // Streaming preview has no closing fence yet.
		"    <at id=all></at> & ![image](key)\n",
	} {
		if got := safeCardMarkdown(source); got != source {
			t.Fatal("code literal changed", got)
		}
	}
}
