export const LIFECYCLE_EVENTS = Object.freeze([
  {
    "id": "DSH-HOST-LIFECYCLE-001",
    "name": "agent-preset/selected",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-002",
    "name": "agent/inbox/spliced",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-003",
    "name": "approval/asked",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-004",
    "name": "approval/decided",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-005",
    "name": "approval/policy",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-006",
    "name": "assistant/attempt",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-007",
    "name": "assistant/message",
    "lifecycle": "surface"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-008",
    "name": "command/done",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-009",
    "name": "command/run",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-010",
    "name": "compaction/end",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-011",
    "name": "compaction/prune",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-012",
    "name": "compaction/start",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-013",
    "name": "compaction/summary",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-014",
    "name": "deliverables/presented",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-015",
    "name": "developer/message",
    "lifecycle": "surface"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-016",
    "name": "feedback/message-delete",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-017",
    "name": "feedback/message-put",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-018",
    "name": "feedback/record",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-019",
    "name": "goal/change",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-020",
    "name": "hook/invoked",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-021",
    "name": "hook/result",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-022",
    "name": "image/offload",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-023",
    "name": "job/registered",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-024",
    "name": "job/settled",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-025",
    "name": "llm/retry",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-026",
    "name": "llm/retry-started",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-027",
    "name": "model/selection",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-028",
    "name": "permission/preset",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-029",
    "name": "plan/mode",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-030",
    "name": "request/context",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-031",
    "name": "request/header",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-032",
    "name": "sandbox/mode",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-033",
    "name": "schedule/change",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-034",
    "name": "session-log-deepseek/delivery-accepted",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-035",
    "name": "session/end-seed",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-036",
    "name": "session/title",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-037",
    "name": "session/title-llm-request",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-038",
    "name": "step/end",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-039",
    "name": "step/start",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-040",
    "name": "subagent/catalog",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-041",
    "name": "subagent/descriptor",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-042",
    "name": "subagent/model-selection-policy",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-043",
    "name": "system/message",
    "lifecycle": "surface"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-044",
    "name": "team/member",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-045",
    "name": "team/message/delivered",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-046",
    "name": "team/message/queued",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-047",
    "name": "team/mode",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-048",
    "name": "team/task",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-049",
    "name": "todo/write",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-050",
    "name": "tool-workflow/agent-end",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-051",
    "name": "tool-workflow/agent-start",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-052",
    "name": "tool-workflow/run-end",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-053",
    "name": "tool-workflow/run-start",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-054",
    "name": "tool/call",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-055",
    "name": "tool/ptc-dispatch",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-056",
    "name": "tool/ptc-dispatch-start",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-057",
    "name": "tool/result",
    "lifecycle": "surface"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-058",
    "name": "turn/end",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-059",
    "name": "turn/start",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-060",
    "name": "user/message",
    "lifecycle": "surface"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-061",
    "name": "web/deepseek-search-llm-request",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-062",
    "name": "working-directory/change",
    "lifecycle": "log-only"
  },
  {
    "id": "DSH-HOST-LIFECYCLE-063",
    "name": "workspace/changes",
    "lifecycle": "log-only"
  }
]);

export const SURFACE_EVENT_TYPES = Object.freeze([
  "assistant/message",
  "developer/message",
  "system/message",
  "tool/result",
  "user/message"
]);
