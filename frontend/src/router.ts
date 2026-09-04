import { createRouter, createWebHistory, type RouterHistory } from "vue-router";
import SessionsPage from "./pages/SessionsPage.vue";

export type Surface = "sessions" | "workflows" | "experts" | "settings";

declare module "vue-router" {
  interface RouteMeta {
    surface?: Surface;
  }
}

export function createAppRouter(history: RouterHistory = createWebHistory()) {
  return createRouter({
    history,
    routes: [
      { path: "/", redirect: "/sessions" },
      { path: "/sessions", name: "sessions", component: SessionsPage, meta: { surface: "sessions" } },
      { path: "/workflows", name: "workflows", component: () => import("./pages/WorkflowsPage.vue"), meta: { surface: "workflows" } },
      { path: "/workflows/:workflowId", name: "workflow-detail", component: () => import("./pages/WorkflowDetailPage.vue"), meta: { surface: "workflows" } },
      { path: "/experts", name: "experts", component: () => import("./pages/ExpertsPage.vue"), meta: { surface: "experts" } },
      { path: "/experts/new", name: "expert-new", component: () => import("./pages/ExpertEditorPage.vue"), meta: { surface: "experts" } },
      { path: "/experts/:expertId", name: "expert-edit", component: () => import("./pages/ExpertEditorPage.vue"), meta: { surface: "experts" } },
      { path: "/expert-teams/new", name: "expert-team-new", component: () => import("./pages/ExpertTeamEditorPage.vue"), meta: { surface: "experts" } },
      { path: "/expert-teams/:teamId", name: "expert-team-edit", component: () => import("./pages/ExpertTeamEditorPage.vue"), meta: { surface: "experts" } },
      { path: "/settings", name: "settings", component: () => import("./pages/SettingsPage.vue"), meta: { surface: "settings" } },
      { path: "/admin/users", name: "users", component: () => import("./pages/UsersPage.vue") },
      { path: "/:pathMatch(.*)*", redirect: "/sessions" },
    ],
  });
}
