import type { AssistantShareConfiguration } from "./api/client";
import tokens from "./design-tokens.css?inline";
import widgetStyles from "./assistant-widget.css?inline";

export interface AssistantWidgetConfiguration {
  styles: string;
  id: string;
  url: string;
  name: string;
  width: string;
  height: number;
  defaultOpen: boolean;
  iconURL?: string;
  openLabel: string;
  closeLabel: string;
}

// Self-contained so the generated snippet can run on a host site without dependencies.
export function installAssistantWidget(configuration: AssistantWidgetConfiguration): HTMLElement | undefined {
  const url = new URL(configuration.url);
  if (!["https:", "http:"].includes(url.protocol) || document.getElementById(configuration.id)) return;
  const host = document.createElement("div");
  host.id = configuration.id;
  const shadow = host.attachShadow({ mode: "open" });
  const style = document.createElement("style");
  style.textContent = configuration.styles;
  const widget = document.createElement("div");
  widget.className = "widget";
  const panel = document.createElement("section");
  panel.className = "panel";
  panel.setAttribute("aria-label", configuration.name);
  panel.style.setProperty("--chat-width", /^[0-9]+px$/.test(configuration.width) ? configuration.width : "400px");
  panel.style.setProperty("--chat-height", `${Math.max(400, Math.min(1600, configuration.height))}px`);
  const bar = document.createElement("div");
  bar.className = "bar";
  const close = document.createElement("button");
  close.type = "button";
  close.className = "close";
  close.textContent = "×";
  close.setAttribute("aria-label", configuration.closeLabel);
  bar.append(close);
  const frame = document.createElement("iframe");
  frame.title = configuration.name;
  frame.src = url.href;
  const launcher = document.createElement("button");
  launcher.type = "button";
  launcher.className = "launcher";
  const showDefaultIcon = () => {
    launcher.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M21 11.5a8.5 8.5 0 0 1-8.5 8.5H4l-3 3V11.5A8.5 8.5 0 0 1 9.5 3h3A8.5 8.5 0 0 1 21 11.5Z"/></svg>';
  };
  showDefaultIcon();
  if (configuration.iconURL) {
    const iconURL = new URL(configuration.iconURL);
    if (iconURL.origin === url.origin) {
      const image = document.createElement("img");
      image.alt = "";
      image.src = iconURL.href;
      image.addEventListener("error", showDefaultIcon, { once: true });
      launcher.replaceChildren(image);
    }
  }
  const setOpen = (open: boolean) => {
    panel.hidden = !open;
    launcher.setAttribute("aria-expanded", String(open));
    launcher.setAttribute("aria-label", open ? configuration.closeLabel : configuration.openLabel);
  };
  launcher.addEventListener("click", () => setOpen(panel.hidden));
  close.addEventListener("click", () => { setOpen(false); launcher.focus(); });
  shadow.addEventListener("keydown", (event) => {
    if ((event as KeyboardEvent).key === "Escape") { setOpen(false); launcher.focus(); }
  });
  panel.append(bar, frame);
  widget.append(panel, launcher);
  shadow.append(style, widget);
  setOpen(configuration.defaultOpen);
  const append = () => { if (!document.getElementById(configuration.id)) document.body.append(host); };
  if (document.body) append();
  else document.addEventListener("DOMContentLoaded", append, { once: true });
  return host;
}

function attribute(value: string): string {
  return value.replace(/&/g, "&amp;").replace(/"/g, "&quot;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

export function assistantEmbedSnippet(origin: string, token: string, name: string, share: AssistantShareConfiguration, labels: { open: string; close: string }): string {
  const base = new URL(origin).origin;
  const url = `${base}/embed/assistant/${encodeURIComponent(token)}`;
  const width = share.width === "100%" || /^[0-9]+px$/.test(share.width) ? share.width : "100%";
  const height = Number.isInteger(share.height) ? Math.max(400, Math.min(1600, share.height)) : 600;
  if (share.embed_type !== "floating") {
    return `<iframe src="${attribute(url)}" title="${attribute(name)}" style="width:${width};height:100%;min-height:${height}px;border:0" loading="lazy"></iframe>`;
  }
  const configuration: AssistantWidgetConfiguration = {
    styles: `${tokens.replace(/:root/g, ":host")}\n${widgetStyles}`,
    id: `assistant-widget-${token}`, url, name, width, height,
    defaultOpen: share.widget_default_open ?? false,
    iconURL: share.widget_icon ? `${base}/api/v1/public/assistants/${encodeURIComponent(token)}/widget-icon` : undefined,
    openLabel: labels.open, closeLabel: labels.close,
  };
  // Escaping '<' prevents configuration text from terminating the host's script element.
  const serialized = JSON.stringify(configuration).replace(/</g, "\\u003c");
  return `<script>(${installAssistantWidget.toString()})(${serialized});</script>`;
}
