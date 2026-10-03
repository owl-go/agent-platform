import { createApp } from "vue";
import AssistantEmbedPreviewPage from "./pages/AssistantEmbedPreviewPage.vue";
import { createAppI18n } from "./i18n";
import "./design-tokens.css";

createApp(AssistantEmbedPreviewPage)
  .use(createAppI18n({ getItem: () => null }, "zh-CN"))
  .mount("#app");
