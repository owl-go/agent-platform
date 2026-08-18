import type { CurrentUser } from "../auth/session";
import type { Surface } from "../router";

const surfaceRoles: Record<Surface, ReadonlySet<string>> = {
  studio: new Set(["platform_administrator", "agent_builder"]),
  workspace: new Set(["platform_administrator", "agent_builder", "agent_user"]),
  operations: new Set(["platform_administrator", "run_operator"]),
};

export function canAccessSurface(user: CurrentUser, teamID: string, surface: Surface): boolean {
  return (user.role_grants ?? []).some((grant) =>
    (!grant.team_id || grant.team_id === teamID) && surfaceRoles[surface].has(grant.role ?? ""),
  );
}

export function selectAccessibleTeam(user: CurrentUser, requestedTeamID: unknown) {
  const teams = user.teams ?? [];
  const requested = typeof requestedTeamID === "string" ? requestedTeamID : "";
  return teams.find((team) => team.id === requested) ?? teams[0];
}
