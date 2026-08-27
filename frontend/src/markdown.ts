import MarkdownIt from "markdown-it";

const markdown = new MarkdownIt({
  breaks: true,
  html: false,
  linkify: true,
  typographer: false,
});

// Agent output is untrusted. Keep links explicit and never load remote images.
markdown.validateLink = (url) => /^(https?:|mailto:|#|\/|\.\.?\/)/i.test(url) || !/^[a-z][a-z\d+.-]*:/i.test(url);
markdown.renderer.rules.image = (tokens, index) => markdown.utils.escapeHtml(tokens[index]?.content ?? "");
markdown.renderer.rules.link_open = (tokens, index, options, _environment, renderer) => {
  tokens[index]?.attrSet("target", "_blank");
  tokens[index]?.attrSet("rel", "noopener noreferrer");
  return renderer.renderToken(tokens, index, options);
};

export function renderMarkdown(value: string): string {
  return markdown.render(value);
}
