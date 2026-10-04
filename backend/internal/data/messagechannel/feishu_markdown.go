package messagechannel

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// Keep Markdown syntax, including code literals, but make model-provided HTML
// and image syntax inert. Feishu's HTML extensions can otherwise mention users
// or embed provider resources. CommonMark parsing identifies actual code spans
// and nested/unfinished fences; regex matching cannot reliably do that.
func feishuMarkdown(source string) string {
	data := []byte(source)
	code := make([]bool, len(data))
	mark := func(start, end int) {
		for i := max(0, start); i < min(end, len(code)); i++ {
			code[i] = true
		}
	}
	document := goldmark.DefaultParser().Parse(text.NewReader(data))
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node.(type) {
		case *ast.FencedCodeBlock, *ast.CodeBlock:
			for i := 0; i < node.Lines().Len(); i++ {
				segment := node.Lines().At(i)
				mark(segment.Start, segment.Stop)
			}
			return ast.WalkSkipChildren, nil
		case *ast.CodeSpan:
			for child := node.FirstChild(); child != nil; child = child.NextSibling() {
				if literal, ok := child.(*ast.Text); ok {
					mark(literal.Segment.Start, literal.Segment.Stop)
				}
			}
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	var out strings.Builder
	out.Grow(len(data))
	for i, b := range data {
		if code[i] {
			out.WriteByte(b)
			continue
		}
		switch b {
		case '&':
			out.WriteString("&amp;")
		case '<':
			out.WriteString("&lt;")
		case '!':
			if i+1 < len(data) && data[i+1] == '[' {
				out.WriteString("&#33;")
			} else {
				out.WriteByte(b)
			}
		default:
			out.WriteByte(b)
		}
	}
	return out.String()
}
