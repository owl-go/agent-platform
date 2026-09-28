import { createRouter, createWebHistory, type RouterHistory } from "vue-router";
import SessionsPage from "./pages/SessionsPage.vue";

export type Surface = "home" | "sessions" | "workflows" | "experts" | "resources" | "ai-creation" | "ai-applications" | "settings";

declare module "vue-router" {
  interface RouteMeta {
    surface?: Surface;
  }
}

export function createAppRouter(history: RouterHistory = createWebHistory()) {
  return createRouter({
    history,
    routes: [
      { path: "/", redirect: "/home" },
      { path: "/home", name: "home", component: () => import("./pages/HomePage.vue"), meta: { surface: "home" } },
      { path: "/sessions", name: "sessions", component: SessionsPage, meta: { surface: "sessions" } },
      { path: "/workflows", name: "workflows", component: () => import("./pages/WorkflowsPage.vue"), meta: { surface: "workflows" } },
      { path: "/workflows/:workflowId", name: "workflow-detail", component: () => import("./pages/WorkflowDetailPage.vue"), meta: { surface: "workflows" } },
      { path: "/experts", name: "experts", redirect: (to) => ({ path: "/resources", query: { ...to.query, tab: to.query.tab === "teams" ? "teams" : "experts" } }) },
      { path: "/experts/new", name: "expert-new", component: () => import("./pages/ExpertEditorPage.vue"), meta: { surface: "resources" } },
      { path: "/experts/:expertId", name: "expert-edit", component: () => import("./pages/ExpertEditorPage.vue"), meta: { surface: "resources" } },
      { path: "/expert-teams/new", name: "expert-team-new", component: () => import("./pages/ExpertTeamEditorPage.vue"), meta: { surface: "resources" } },
      { path: "/expert-teams/:teamId", name: "expert-team-edit", component: () => import("./pages/ExpertTeamEditorPage.vue"), meta: { surface: "resources" } },
      { path: "/resources", name: "resources", component: () => import("./pages/ResourceCenterPage.vue"), meta: { surface: "resources" } },
      { path: "/resources/skills/:skillId", name: "skill-detail", component: () => import("./pages/SkillDetailPage.vue"), meta: { surface: "resources" } },
      { path: "/knowledge-bases", redirect: (to) => ({ path: "/resources", query: { ...to.query, tab: "knowledge" } }) },
      { path: "/ai-creation", redirect: "/ai-apps/image-creation" },
      {
        path: "/ai-apps",
        component: () => import("./pages/AIApplicationsPage.vue"),
        meta: { surface: "ai-applications" },
        children: [
          { path: "", redirect: "image-creation" },
          { path: "image-creation", name: "image-creation", component: () => import("./pages/ImageGenerationPage.vue"), meta: { surface: "ai-applications" } },
          { path: "assistants", name: "smart-assistants", component: () => import("./pages/SmartAssistantsPage.vue"), meta: { surface: "ai-applications" } },
          { path: "assistants/:assistantId", name: "smart-assistant-detail", component: () => import("./pages/SmartAssistantDetailPage.vue"), meta: { surface: "ai-applications" } },
          { path: "assistants/:assistantId/conversations/:conversationId", name: "smart-assistant-conversation", component: () => import("./pages/SmartAssistantConversationPage.vue"), meta: { surface: "ai-applications" } },
          { path: "knowledge-bases", redirect: () => ({ path: "/resources", query: { tab: "knowledge" } }) },
          { path: "digital-humans", name: "digital-humans", component: () => import("./pages/DigitalHumansPage.vue"), meta: { surface: "ai-applications" } },
          { path: "digital-humans/:digitalHumanId", name: "digital-human-detail", component: () => import("./pages/DigitalHumanDetailPage.vue"), meta: { surface: "ai-applications" } },
        ],
      },
      { path: "/ai-creation/image-generation", redirect: "/ai-apps/image-creation" },
      { path: "/settings", name: "settings", component: () => import("./pages/SettingsPage.vue"), meta: { surface: "settings" } },
      { path: "/admin/users", name: "users", component: () => import("./pages/UsersPage.vue") },
      { path: "/:pathMatch(.*)*", redirect: "/home" },
    ],
  });
}
