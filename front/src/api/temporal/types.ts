export type TemporalOwnerType = "user" | "character"
export type TemporalTimezoneMode = "follow_device" | "fixed" | "follow_user" | "narrative"
export type TemporalAnchorTimeKind = "instant" | "local_date" | "annual_date" | "local_datetime" | "range" | "recurring" | "derived"

export interface TemporalProfile {
  id: string
  ownerType: TemporalOwnerType
  ownerId: string
  timezoneMode: TemporalTimezoneMode
  timezone: string
  locale: string
  calendarSystem: string
  weekStart: number
  holidayRegion: string
  hemisphere: "north" | "south" | "unknown"
  daypartConfigJson: string
  quietHoursJson: string
  autoDetectTimezone: boolean
  travelMode: boolean
  awarenessLevel: number
  source: string
  confidence: number
  pendingTimezoneSuggestion?: string
  enabled: boolean
  holidayAwareness: boolean
  daypartAwareness: boolean
  anniversaryAwareness: boolean
  memoryResonance: boolean
  allowSharedDateMention: boolean
  version: number
  createdAtUtc: string
  updatedAtUtc: string
}

export interface CivilTimeSnapshot {
  timezone: string
  localTime: string
  weekday: string
  daypart: string
  season: string
  offsetSeconds: number
  isWeekend: boolean
  isWorkday: boolean
}

export interface CalendarEvent {
  id: string
  name: string
  localDate: string
  kind: string
  region?: string
  source: string
}

export interface AnchorOccurrence {
  id: string
  type: string
  title: string
  distanceDays: number
  salience: number
}

export interface TemporalSnapshot {
  version: string
  nowUtc: string
  userTime: CivilTimeSnapshot
  characterTime: CivilTimeSnapshot
  relationshipTime?: RelationshipTimeContext | SnapshotField<RelationshipTimeContext> | null
  schedule: { currentState?: string; busy: boolean }
  calendarEvents?: CalendarEvent[]
  salientAnchors: AnchorOccurrence[]
  signals: { timezoneDiffers: boolean; quietHours: boolean; dayChanged: boolean }
  policy: { mentionTime: string; allowProactive: boolean; maxTemporalMentions: number }
  generatedAt: string
}

export interface SnapshotField<T> {
  status?: "ready" | "unavailable" | "disabled" | "error" | string
  value?: T
  data?: T
  source?: string
  version?: string
  error?: string
}

export type RelationshipTimeSensitivity = "conservative" | "balanced" | "expressive"
export type ReunionKind = "global_return" | "relationship_reconnect" | "reply_to_recent_proactive" | string
export type ReunionLevel = "none" | "noticeable" | "long" | "extended" | "dormant" | string
export type ReunionState = "pending" | "claimed" | "handled" | "suppressed" | "expired" | string

export interface RelationshipTimeSettings {
  characterId: string
  enabled: boolean
  reunionEnabled: boolean
  sensitivity: RelationshipTimeSensitivity
  allowMemoryRecall: boolean
  allowRelationshipAge: boolean
  allowReunionMention: boolean
  allowProactiveReference: boolean
  maxMentionSentences: number
  updatedAt?: string
}

export interface ReunionContext {
  episodeId: string
  kind: ReunionKind
  level: ReunionLevel
  state: ReunionState
  relationshipGapSeconds: number
  globalGapSeconds: number
  expectedGapSeconds: number
  normalizedGap: number
  claimedByInteractionId?: string
  claimExpiresAt?: string
  shouldExpress: boolean
}

export interface RelationshipTimeContext {
  version: string
  userId: string
  characterId: string
  nowUtc: string
  firstInteractionAt?: string
  globalLastCommittedAt?: string
  relationshipLastCommittedAt?: string
  lastSuccessfulExchangeAt?: string
  lastAssistantContactAt?: string
  globalGapSeconds: number
  relationshipGapSeconds: number
  expectedGapSeconds: number
  gapDeviationScore: number
  normalizedGap: number
  relationshipAgeDays: number
  interactionCount: number
  sessionCount: number
  reunion?: ReunionContext
  continuityScore: number
  reacclimationTurnsLeft: number
  effectiveTension: number
  storedTension: number
  hasRecentAssistantContact: boolean
  diagnostics?: string[]
}

export interface ReunionEpisode {
  id: string
  userId: string
  characterId: string
  reunionKind: ReunionKind
  reunionLevel: ReunionLevel
  status: ReunionState
  previousRelationshipInteractionAtUtc?: string
  previousGlobalInteractionAtUtc?: string
  detectedAtUtc: string
  relationshipGapSeconds: number
  globalGapSeconds: number
  expectedGapSeconds: number
  normalizedGap: number
  deviationScore: number
  continuityBefore: number
  claimInteractionId?: string
  claimExpiresAtUtc?: string
  handledInteractionId?: string
  handledAtUtc?: string
  suppressionReason?: string
  createdAtUtc?: string
  updatedAtUtc?: string
}

export interface RelationshipTimeDiagnostics {
  available?: boolean
  enabled?: boolean
  state?: RelationshipTimeContext
  relationshipTime?: RelationshipTimeContext | SnapshotField<RelationshipTimeContext> | null
  episodes?: ReunionEpisode[]
  diagnostics?: string[]
  [key: string]: unknown
}

export interface TemporalAnchor {
  id: string
  scopeType: "user" | "character" | "relationship"
  userId: string
  characterId: string
  anchorType: string
  title: string
  description: string
  timeKind: TemporalAnchorTimeKind
  instantAtUtc?: string
  endAtUtc?: string
  localDate?: string
  localTime?: string
  timezone?: string
  rrule?: string
  durationSeconds: number
  preWindowSeconds: number
  postWindowSeconds: number
  importance: number
  confidence: number
  sensitivityLevel: string
  allowPromptMention: boolean
  allowProactiveMention: boolean
  requiresConfirmation: boolean
  source: string
  sourceRef?: string
  payloadJson?: string
  status: "candidate" | "active" | "disabled" | "archived"
  nextOccurrenceAtUtc?: string
  lastOccurrenceAtUtc?: string
  createdAtUtc: string
  updatedAtUtc: string
}

export interface TemporalDiagnostics {
  snapshot: TemporalSnapshot
  core: { featureFlags: { temporalCoreEnabled: boolean; relationshipTimeEnabled: boolean }; clockSource: string; tzdb: string }
  relationshipTime?: RelationshipTimeContext | SnapshotField<RelationshipTimeContext> | null
  promptSections: Array<{ type: string; content: string }>
  commitEffects: unknown[]
  recentEvents: unknown[]
	metrics: {
		snapshotCount: number
		snapshotErrors: number
		snapshotLatencyTotalMs: number
		snapshotLatencyAverageMs: number
		anchorEvents: number
		anchorDeduplicated: number
		anchorRecoveryExpired: number
		proactiveCandidates: number
		proactiveCandidateErrors: number
		memoryRerankCandidates: number
		memoryTemporalBoostMicros: number
	}
  diagnostics: string[]
  snapshotVersion: string
}
