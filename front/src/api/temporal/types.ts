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
  relationshipTime?: unknown
  schedule: { currentState?: string; busy: boolean }
  calendarEvents?: CalendarEvent[]
  salientAnchors: AnchorOccurrence[]
  signals: { timezoneDiffers: boolean; quietHours: boolean; dayChanged: boolean }
  policy: { mentionTime: string; allowProactive: boolean; maxTemporalMentions: number }
  generatedAt: string
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
  relationshipTime?: unknown
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
