export type TeamMemberSource = "center" | "device";

export type TeamMemberDraft = {
  source: TeamMemberSource;
  deviceId: string;
  roleId: string;
  role: "lead" | "member";
  roleRevision?: number;
};

export function buildTeamMemberPayload(draft: TeamMemberDraft): Record<string, unknown> {
  const payload: Record<string, unknown> = {
    memberType: "agent",
    memberId: draft.roleId,
    role: draft.role,
    responderMode: draft.role === "lead" ? "lead" : "mention_or_lead",
    capabilities: draft.role === "lead" ? ["*"] : [],
  };
  if (draft.source === "device") {
    payload.ownerType = "device";
    payload.ownerId = draft.deviceId;
    payload.invocationMode = "role";
    if (draft.roleRevision && draft.roleRevision > 0) payload.roleRevision = draft.roleRevision;
  }
  return payload;
}
