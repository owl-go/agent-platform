import { describe, expect, it } from "vitest";
import type { CurrentUser } from "../auth/session";
import { canAccessSurface, selectAccessibleTeam } from "./access";

const user: CurrentUser = {
  user_id: "user-1",
  role_grants: [{ role: "agent_builder", team_id: "team-a" }, { role: "run_operator" }],
  teams: [{ id: "team-a", slug: "alpha", name: "Alpha" }, { id: "team-b", slug: "beta", name: "Beta" }],
};

describe("Team access presentation", () => {
  it("selects only a Team returned by the authenticated bootstrap", () => {
    expect(selectAccessibleTeam(user, "team-b")?.id).toBe("team-b");
    expect(selectAccessibleTeam(user, "foreign")?.id).toBe("team-a");
  });

  it("uses Organization and Team Role Grants only for navigation hints", () => {
    expect(canAccessSurface(user, "team-a", "studio")).toBe(true);
    expect(canAccessSurface(user, "team-b", "studio")).toBe(false);
    expect(canAccessSurface(user, "team-b", "operations")).toBe(true);
  });
});
