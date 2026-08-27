import { createRouter, createWebHistory, type RouterHistory } from "vue-router";
import SessionsPage from "./pages/SessionsPage.vue";
import WorkflowsPage from "./pages/WorkflowsPage.vue";
import WorkflowDetailPage from "./pages/WorkflowDetailPage.vue";
import ExpertsPage from "./pages/ExpertsPage.vue";
import SettingsPage from "./pages/SettingsPage.vue";
import UsersPage from "./pages/UsersPage.vue";

export type Surface = "sessions" | "workflows" | "experts" | "settings";

export function createAppRouter(history: RouterHistory = createWebHistory()) {
  return createRouter({
    history,
    routes: [
      { path: "/", redirect: "/sessions" },
      { path: "/sessions", name: "sessions", component: SessionsPage },
      { path: "/workflows", name: "workflows", component: WorkflowsPage },
      { path: "/workflows/:workflowId", name: "workflow-detail", component: WorkflowDetailPage },
      { path: "/experts", name: "experts", component: ExpertsPage },
      { path: "/settings", name: "settings", component: SettingsPage },
      { path: "/admin/users", name: "users", component: UsersPage },
      { path: "/:pathMatch(.*)*", redirect: "/sessions" },
    ],
  });
}
