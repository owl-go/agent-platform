import { createRouter, createWebHistory, type RouterHistory } from "vue-router";
import OperationsPage from "./pages/OperationsPage.vue";
import StudioPage from "./pages/StudioPage.vue";
import WorkspacePage from "./pages/WorkspacePage.vue";

export type Surface = "studio" | "workspace" | "operations";

export function createAppRouter(history: RouterHistory = createWebHistory()) {
  return createRouter({
    history,
    routes: [
      { path: "/", redirect: "/workspace" },
      { path: "/studio", name: "studio", component: StudioPage },
      { path: "/workspace", name: "workspace", component: WorkspacePage },
      { path: "/operations", name: "operations", component: OperationsPage },
      { path: "/:pathMatch(.*)*", redirect: "/workspace" },
    ],
  });
}
