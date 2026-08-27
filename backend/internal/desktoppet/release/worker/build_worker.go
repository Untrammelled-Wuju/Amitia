package worker

// Release builds are synchronous operations owned by release.ReleaseService.
//
// The previous alternate build worker was not wired into the server lifecycle and
// implemented a second state-transition path over the same build-operation
// records. It is intentionally retired. This package remains for the release
// outbox dispatcher, which is an independent delivery concern.
