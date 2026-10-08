# Mobile Notification Runtime

This document describes the single mobile notification path for Android and iOS.

## Ownership

There is one notification state source: `backend/internal/notificationruntime`.

- Local deployment: Core prefers the platform Native Bridge. No APNs/FCM/vendor server secret is required on the device.
- Cloud deployment: Cloud Core owns remote delivery, durable outbox/retry and provider credentials.
- Flutter owns device registration, preferences, foreground presence, deep links and reply routing.
- Android/iOS native code owns OS presentation.

Do not reintroduce the legacy mobile `/api/notifications/settings|subscribe|unsubscribe|status|test` flow. Mobile code uses `/api/notifications/devices*`, `presence`, `live-activities/token` and `calls/*`. Browser-local notification preferences are isolated under `/api/browser-notifications/*`; they are not a mobile Push state source.

## Cloud Core providers

Secrets live only on Cloud Core. Copy `backend/.env.notification.example` into the deployment secret store.

Provider readiness is available from:

`GET /api/notifications/capabilities`

The response exposes `pushProviders`, `nativeDataPushProviders`, `deliveryMode` and `cloudOwned`. `deliveryMode=remote` is expected on Cloud Core; `deliveryMode=native` is expected for local Core with a mobile Native Bridge. `nativeDataPushProviders` is the server-side data-plane capability and must be intersected with the device-reported `nativeDataProviders`; token presence alone never enables pass-through delivery.

Remote providers:

- FCM HTTP v1: Android default/fallback.
- APNs: iOS message/reminder/task fallback.
- APNs VoIP: iOS incoming call wakeup + CallKit.
- APNs Live Activity: remote start/update/end of lock-screen/Dynamic Island execution state.
- Xiaomi MiPush, Huawei Push Kit, HONOR Push, OPPO/HeyTap, vivo Push: Android domestic reachability.

Cloud delivery is durable. A notification is persisted to `notification_outbox` before network delivery. Workers atomically claim due rows, retry retryable network/408/425/429/5xx errors, recover `processing` rows after restart and stop after TTL.

## Android client channel configuration

These values are client/channel configuration, not Cloud Core server secrets:

- `AMITIA_FCM_APPLICATION_ID`
- `AMITIA_FCM_API_KEY`
- `AMITIA_FCM_PROJECT_ID`
- `AMITIA_FCM_SENDER_ID`
- `AMITIA_XIAOMI_APP_ID`
- `AMITIA_XIAOMI_APP_KEY`
- `AMITIA_HUAWEI_APP_ID`
- `AMITIA_HONOR_APP_ID`
- `AMITIA_OPPO_APP_KEY`
- `AMITIA_OPPO_APP_SECRET`
- `AMITIA_VIVO_APP_ID`
- `AMITIA_VIVO_API_KEY`

Vendor SDKs are optional channel integrations. `VendorPushBootstrap` detects an integrated SDK at runtime and uploads the resulting vendor token together with FCM, so Cloud Core can prefer the device vendor channel and retain FCM as fallback. Proprietary vendor AAR/JAR files may be placed in `mobile_app/android/app/libs`; the default project does not require every vendor SDK.

For full native-data rendering (the same `MessagingStyle`, `CallStyle` and task progress renderer used by FCM), enable only adapters whose official SDK is actually linked:

- Xiaomi: place the official MiPush AAR in `app/libs` (auto-detected) or set `AMITIA_XIAOMI_NATIVE_DATA=1`.
- Huawei: set `AMITIA_HUAWEI_PUSH_DEPENDENCY` to the official Maven coordinate or provide its AAR, then set `AMITIA_HUAWEI_NATIVE_DATA=1` when filename auto-detection is not sufficient.
- HONOR: set `AMITIA_HONOR_PUSH_DEPENDENCY` (current official Maven coordinate is `com.hihonor.mcs:push:<version>`) or provide its AAR, then set `AMITIA_HONOR_NATIVE_DATA=1` when needed. `AMITIA_HONOR_SDK_PACKAGE=hihonor` is the default for current SDKs; set it to `honor` only for legacy packages exposing `com.honor.push.sdk`.
- OPPO/HeyTap: set `AMITIA_OPPO_PUSH_DEPENDENCY` or place the official Push AAR in `app/libs`, and set `AMITIA_OPPO_NATIVE_DATA=1` only after the application has OPPO pass-through permission. Android Q+ and pre-Q callbacks converge through `VendorInboundBridge`.
- vivo: set `AMITIA_VIVO_PUSH_DEPENDENCY` or place the official Push AAR in `app/libs`, and set `AMITIA_VIVO_NATIVE_DATA=1` only for an SDK/application configuration that exposes transmission callbacks.

The client reports `nativeDataProviders` during device registration only when the matching receiver is actually compiled and enabled. Cloud Core sends Xiaomi/Huawei/HONOR pass-through data only when both client and server data-plane capability are true. OPPO/vivo client receivers are implemented, but Cloud Core deliberately advertises their server data-plane capability as false until a verified server pass-through API/entitlement is configured; it therefore keeps the vendor-system notification path plus FCM fallback instead of silently sending undeliverable data.

## iOS presentation

Message notifications:

Cloud Core APNs -> Notification Service Extension -> `INSendMessageIntent` -> communication-style system notification.

The extension preserves the original `AMITIA_MESSAGE` category, thread identifier, userInfo/deep link, sound and badge after the communication upgrade, so text reply remains available.

Local Core uses the same `INSendMessageIntent` upgrade in `IOSNativeHost`.

Execution notifications:

- local mode: ActivityKit creates/updates/ends the Live Activity directly through the Native Bridge;
- cloud mode: APNs uses push-to-start/update tokens;
- active execution state is rebuilt from persisted assistant turns plus durable ConversationStream events after Core restart;
- a two-minute heartbeat refreshes active execution presentation in both local and cloud modes; Live Activity state becomes stale after five minutes without refresh;
- device registration catches up at most one current run, while an ActivityKit update-token callback catches up that exact run to avoid cross-run token races;
- repeated push-to-start catch-up is throttled and terminal runs are never resurrected;
- a newer run in the same conversation ends an older still-active Live Activity;
- Live Activity content carries locale, appearance and agent identity, and terminal completed state remains visible briefly while interrupted/cancelled state dismisses immediately;
- failed/unavailable Live Activity delivery falls back to a normal visible APNs execution notification;
- invalid push-to-start/update tokens are cleared/ended instead of being reused forever.

Incoming calls:

- cloud mode: APNs VoIP -> PushKit -> CallKit;
- duplicate VoIP pushes for the same call ID are ignored;
- an invalid VoIP token is cleared and an incoming call falls back to a visible ordinary APNs alert/deep link until PushKit rotates a valid token;
- local mode: Native Bridge -> the same CallKit provider.

## Android presentation

- messages: MessagingStyle + RemoteInput reply;
- reminders/proactive messages: reminder channel; backend `reminder:` requests emit `reminder.triggered`, other `Source=proactive` turns emit `proactive.message`; both honor the device `reminderPushEnabled` preference;
- incoming calls: CallStyle + answer/decline/full-screen intent;
- execution: Android 16 ProgressStyle when available, compatible ongoing progress notification otherwise;
- revision suppression prevents stale task updates from overwriting a newer state.

## Device registration and privacy

The authenticated actor device ID is the notification device identity in both local and cloud modes. The client does not invent a second Cloud device identity.

Registration preserves existing preferences and tokens when a field is omitted. Explicit token invalidation uses `clearTokens`. Provider-invalid tokens are also cleared automatically. Locale and current light/dark appearance are registered for Live Activity presentation. On iOS, ordinary APNs and PushKit VoIP registration keep separate invalidation state so one channel never clears the other.

Android manufacturer SDK refresh no longer runs merely because a Flutter engine attached. Vendor registration is deferred to notification initialization when system notifications are already enabled, or until the user grants notification permission. A product-level privacy-consent gate should remain the outer gate if/when the application introduces a persisted first-launch consent state.

Deployment handoff is notification-aware. Before Local ↔ Cloud or Cloud A ↔ Cloud B transitions, the client best-effort revokes the current notification endpoint on the old Core, suppresses concurrent token refreshes, switches the backend transport, then registers on the newly active Core. Failed deployment changes roll back the previous transport/configuration and re-register there.

Preview modes:

- `full`: sender + body;
- `sender_only`: sender only;
- `hidden`: generic private text.

Foreground presence suppresses redundant message Push for the conversation currently visible on that device.

## iOS targets

Runner embeds both app extensions:

- `AmitiaLiveActivity`
- `AmitiaNotificationService`

Runner declares `NSSupportsLiveActivities`, `INSendMessageIntent`, remote notification/VoIP background modes, Push entitlement, communication notification entitlement and the shared app group.

## Operational rules

Provider credentials must never be logged or returned by APIs. Device push tokens are persisted server-side but omitted from device JSON responses.

Use `/api/notifications/devices/{deviceId}/test` for a real end-to-end test Push. The old local-only test endpoint is not part of the mobile path.

Provider availability is configuration-dependent. A false capability means that provider is not configured on that Core; it is not a reason to disable the whole notification runtime when another provider/native bridge is available.
