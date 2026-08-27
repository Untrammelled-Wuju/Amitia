package build

// Release build orchestration is intentionally owned by release.ReleaseService.
//
// This package contains only the lower-level snapshot, sequencing, lease,
// journal, and package-writing primitives that are shared with recovery. The
// previous alternate orchestration implementation was never wired into production
// and duplicated the canonical release state machine, so it is deliberately
// retired to keep a single release build authority for the v2 freeze.
