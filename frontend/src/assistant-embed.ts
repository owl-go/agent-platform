import { createApp } from "vue";
import PublicAssistantConversationPage from "./pages/PublicAssistantConversationPage.vue";
import { createPublicAssistantApi } from "./api/publicAssistant";
import { createAppI18n } from "./i18n";
import tokens from "./design-tokens.css?inline";
import styles from "./styles.css?inline";
import controls from "element-plus/dist/index.css?inline";

const root = document.getElementById("app")!;
const style = document.createElement("style");
style.textContent = `${controls}\n${tokens}\n${styles}`;
document.head.appendChild(style);
const app = createApp(PublicAssistantConversationPage, {
  api: createPublicAssistantApi(root.dataset.assistantApi!), width: root.dataset.width || "100%",
  height: Number(root.dataset.height) || 600, embedded: window.self !== window.top,
});
// Embedded browsers may deny access to third-party localStorage.
app.use(createAppI18n({ getItem: () => null }, navigator.language));
app.mount(root);
