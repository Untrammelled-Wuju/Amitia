export type OwnedRuntimeStateEnvelope = {
  ownerId?: string;
  roleId?: string;
  updatedAt?: string;
  psyche?: {
    valence?: number;
    dominance?: number;
    arousal?: number;
    moodValence?: number;
    moodArousal?: number;
    stress?: number;
    fatigue?: number;
    moodPressure?: number;
    socialLoad?: number;
    recoveryHours?: number;
    needs?: Record<string, number>;
  };
  relationship?: {
    trust?: number;
    familiarity?: number;
    security?: number;
    tension?: number;
    repairConfidence?: number;
    boundary?: number;
  };
  needs?: Array<{ kind?: string; level?: number; baseline?: number; updatedAt?: string }>;
  beliefs?: Array<{ key?: string; value?: string; confidence?: number; conflicted?: boolean }>;
};

export function projectOwnedRuntimeState(state: OwnedRuntimeStateEnvelope) {
  const psyche = state.psyche ?? {};
  const relationship = state.relationship ?? {};
  const valence = number(psyche.valence, 0.5);
  const stress = number(psyche.stress, 0);
  const needs = Object.fromEntries(
    (state.needs ?? [])
      .filter((item) => typeof item.kind === "string" && item.kind)
      .map((item) => [item.kind as string, number(item.level, 0.5)]),
  );
  return {
    emotion: {
      positive: valence,
      negative: 1 - valence,
      arousal: number(psyche.arousal, 0.5),
      dominance: number(psyche.dominance, 0.5),
      updatedAt: state.updatedAt,
    },
    mood: {
      valence: number(psyche.moodValence, 0.5),
      tension: number(psyche.moodArousal, 0.5),
      pad: "",
      updatedAt: state.updatedAt,
    },
    stress,
    energy: 1 - number(psyche.fatigue, 0.3),
    needs,
    beliefs: (state.beliefs ?? []).map((belief) => ({
      key: String(belief.key ?? ""),
      value: String(belief.value ?? ""),
      confidence: number(belief.confidence, 0),
      conflicted: Boolean(belief.conflicted),
    })),
    relationship: {
      trust: number(relationship.trust, 0.5),
      familiarity: number(relationship.familiarity, 0),
      security: number(relationship.security, 0.5),
      tension: number(relationship.tension, 0),
      repairConfidence: number(relationship.repairConfidence, 0.5),
      boundary: number(relationship.boundary, 0.5),
    },
    affectLabel: stress > 0.55 ? "紧张" : valence > 0.55 ? "积极" : valence < 0.45 ? "消极" : "平静",
    collectedAt: state.updatedAt ?? "",
  };
}

function number(value: unknown, fallback: number): number {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}
