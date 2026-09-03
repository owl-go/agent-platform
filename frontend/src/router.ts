import { createRouter, createWebHistory, type RouterHistory } from "vue-router";
import SessionsPage from "./pages/SessionsPage.vue";

export type Surface = "sessions" | "workflows" | "experts" | "settings";

export function createAppRouter(history: RouterHistory = createWebHistory()) {
  return createRouter({
    history,
    routes: [
      { path: "/", redirect: "/sessions" },
      { path: "/sessions", name: "sessions", component: SessionsPage },
      { path: "/workflows", name: "workflows", component: () => import("./pages/WorkflowsPage.vue") },
      { path: "/workflows/:workflowId", name: "workflow-detail", component: () => import("./pages/WorkflowDetailPage.vue") },
      { path: "/experts", name: "experts", component: () => import("./pages/ExpertsPage.vue") },
      { path: "/experts/new", name: "expert-new", component: () => import("./pages/ExpertEditorPage.vue") },
      { path: "/experts/:expertId", name: "expert-edit", component: () => import("./pages/ExpertEditorPage.vue") },
      { path: "/expert-teams/new", name: "expert-team-new", component: () => import("./pages/ExpertTeamEditorPage.vue") },
      { path: "/expert-teams/:teamId", name: "expert-team-edit", component: () => import("./pages/ExpertTeamEditorPage.vue") },
      { path: "/settings", name: "settings", component: () => import("./pages/SettingsPage.vue") },
      { path: "/admin/users", name: "users", component: () => import("./pages/UsersPage.vue") },
      { path: "/:pathMatch(.*)*", redirect: "/sessions" },
    ],
  });
}
