// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package revisioncommit

// The pre-Inbox direct RevisionBridge implementation was retired after the
// canonical Processing Outbox -> ActionRevision Bridge Inbox -> BridgeProcessor
// pipeline became the sole production writer. Keeping a second callable bridge
// here would reintroduce dual write authority and bypass the lease/retry/
// idempotency guarantees implemented by BridgeProcessor.
