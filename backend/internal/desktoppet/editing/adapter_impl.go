package editing

// Production adapters are intentionally wired at the application boundary in
// backend/cmd/server/editing_ports.go. Keeping raw DB-backed generation,
// processing, or quality adapters inside the editing domain would reintroduce
// a second execution authority and bypass the canonical workers/services.
//
// This file remains as a package-level compatibility placeholder so downstream
// integrations can replace it directly without a separate delete operation.
