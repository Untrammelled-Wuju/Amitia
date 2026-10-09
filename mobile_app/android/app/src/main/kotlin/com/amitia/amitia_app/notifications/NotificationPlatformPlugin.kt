package com.amitia.amitia_app.notifications

import android.content.Context
import android.content.Intent
import android.content.pm.ShortcutManager
import android.net.Uri
import android.util.Log
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.provider.Settings
import androidx.core.app.NotificationManagerCompat
import com.amitia.amitia_app.MainActivity
import io.flutter.embedding.engine.plugins.FlutterPlugin
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import java.lang.ref.WeakReference

class NotificationPlatformPlugin : FlutterPlugin, MethodChannel.MethodCallHandler {
    companion object {
        const val CHANNEL = "com.amitia.notifications/control"
        private const val PREFS = "amitia_notification_interactions"
        private const val PENDING = "pending"
        private const val VENDOR_PUSH_OPT_IN = "vendor_push_opt_in"
        private val scenarioHandler = Handler(Looper.getMainLooper())
        private var scenarioRunnable: Runnable? = null
        private var scenarioRunId: String? = null
        @Volatile private var current: WeakReference<NotificationPlatformPlugin>? = null

        fun emitToken(token: String) {
            current?.get()?.channel?.invokeMethod("pushTokenChanged", mapOf("provider" to "fcm", "token" to token))
        }

        fun emitVendorToken(provider: String, token: String) {
            current?.get()?.channel?.invokeMethod(
                "pushTokenChanged",
                mapOf("provider" to provider, "token" to token),
            )
        }

        fun emitVendorInvalidated(provider: String) {
            current?.get()?.channel?.invokeMethod(
                "pushTokenChanged",
                mapOf("provider" to provider, "token" to null, "invalidated" to true),
            )
        }

        fun emitCallEnded(
            context: Context,
            callId: String,
            conversationId: String,
            reason: String,
        ) {
            if (callId.isBlank()) return
            val instance = current?.get() ?: return
            instance.channel?.invokeMethod(
                "notificationInteraction",
                mapOf(
                    "source" to "callEnded",
                    "callId" to callId,
                    "conversationId" to conversationId,
                    "reason" to reason,
                ),
            )
        }

        fun handleIntent(context: Context, intent: Intent?) {
            if (intent == null) return
            val deepLink = intent.getStringExtra("amitia.deepLink") ?: intent.dataString
            val reply = intent.getStringExtra("amitia.reply")
            val parsed = deepLink?.let { runCatching { android.net.Uri.parse(it) }.getOrNull() }
            val conversationId = intent.getStringExtra("amitia.conversationId")
                ?.takeIf { it.isNotBlank() }
                ?: parsed
                    ?.takeIf { it.scheme == "amitia" && it.host == "chat" }
                    ?.pathSegments
                    ?.firstOrNull()
                    ?.takeIf { it.isNotBlank() }
            if (deepLink.isNullOrBlank() && reply.isNullOrBlank()) return
            if (!conversationId.isNullOrBlank()) {
                NotificationRenderer.clearConversation(context.applicationContext, conversationId)
            }
            val callId = parsed
                ?.takeIf { it.scheme == "amitia" && it.host == "call" }
                ?.getQueryParameter("call")
                ?.takeIf { it.isNotBlank() }
            val callAction = parsed
                ?.takeIf { it.scheme == "amitia" && it.host == "call" }
                ?.getQueryParameter("action")
            if (!callId.isNullOrBlank() && (callAction == "answer" || callAction == "decline")) {
                NotificationRenderer.clearCall(context.applicationContext, callId)
            }
            val payload = mutableMapOf<String, Any?>(
                "deepLink" to deepLink,
                "reply" to reply,
                "conversationId" to conversationId,
                "source" to if (reply.isNullOrBlank()) "notificationTap" else "notificationReply",
            )
            val instance = current?.get()
            if (instance?.channel != null) {
                instance.channel?.invokeMethod("notificationInteraction", payload)
                return
            }
            context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
                .edit()
                .putString(PENDING, org.json.JSONObject(payload).toString())
                .apply()
        }
    }

    private lateinit var appContext: Context
    internal var channel: MethodChannel? = null

    override fun onAttachedToEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        appContext = binding.applicationContext
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            runCatching {
                val shortcuts = appContext.getSystemService(Context.SHORTCUT_SERVICE) as ShortcutManager
                val legacy = shortcuts.getShortcuts(
                    ShortcutManager.FLAG_MATCH_DYNAMIC or
                        ShortcutManager.FLAG_MATCH_PINNED or
                        ShortcutManager.FLAG_MATCH_CACHED,
                ).map { it.id }.filter { it.startsWith("amitia-chat-") }
                if (legacy.isNotEmpty()) {
                    shortcuts.removeDynamicShortcuts(legacy)
                    shortcuts.removeLongLivedShortcuts(legacy)
                }
            }.onFailure { Log.w("AmitiaFloatingChat", "Legacy bubble cleanup failed", it) }
        }
        NotificationRenderer.ensureChannels(appContext)
        channel = MethodChannel(binding.binaryMessenger, CHANNEL).also { it.setMethodCallHandler(this) }
        current = WeakReference(this)
    }

    override fun onDetachedFromEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        channel?.setMethodCallHandler(null)
        channel = null
        if (current?.get() === this) current = null
    }

    override fun onMethodCall(call: MethodCall, result: MethodChannel.Result) {
        when (call.method) {
            "initialize" -> {
                NotificationRenderer.ensureChannels(appContext)
                val configured = PushBootstrap.initialize(appContext)
                if (vendorPushOptedIn() &&
                    NotificationManagerCompat.from(appContext).areNotificationsEnabled()
                ) {
                    VendorPushBootstrap.refresh(appContext)
                }
                PushBootstrap.refreshToken(appContext) { token ->
                    result.success(capabilities(configured, token))
                }
            }
            "getCapabilities" -> result.success(capabilities(PushBootstrap.configured(), PushBootstrap.currentToken(appContext)))
            "getPushToken" -> PushBootstrap.refreshToken(appContext) { token ->
                result.success(mapOf("provider" to "fcm", "token" to token))
            }
            "requestPermission" -> requestPermission(result)
            "setPushEnabled" -> {
                val enabled = call.argument<Boolean>("enabled") ?: true
                setPushEnabled(enabled)
                result.success(true)
            }
            "consumeInitialInteraction" -> result.success(consumePendingInteraction())
            "endCall" -> {
                val callId = call.argument<String>("callId").orEmpty()
                NotificationRenderer.clearCall(appContext, callId)
                result.success(true)
            }
            "clearExecutionNotifications" -> {
                NotificationRenderer.clearAllExecution(appContext)
                result.success(true)
            }
            "clearMessageNotifications" -> {
                NotificationRenderer.clearAllMessages(appContext)
                result.success(true)
            }
            "clearCallNotifications" -> {
                NotificationRenderer.clearAllCalls(appContext)
                result.success(true)
            }
            "clearReminderNotifications" -> {
                NotificationRenderer.clearAllReminders(appContext)
                result.success(true)
            }
            "floatingBubbleStatus" -> {
                val state = FloatingChatBubbleService.status(appContext)
                Log.i("AmitiaFloatingChat", "method floatingBubbleStatus enabled=${state["enabled"]} active=${state["active"]} permission=${state["permissionGranted"]}")
                result.success(state)
            }
            "floatingBubbleEnable" -> {
                Log.i("AmitiaFloatingChat", "method floatingBubbleEnable called")
                try {
                    val started = FloatingChatBubbleService.enable(appContext)
                    result.success(FloatingChatBubbleService.status(appContext) + mapOf("started" to started))
                } catch (error: Exception) {
                    result.error("FLOATING_BUBBLE_START_FAILED", error.message, null)
                }
            }
            "floatingBubbleConfigure" -> {
                val enabled = call.argument<Boolean>("previewEnabled") ?: true
                result.success(FloatingChatBubbleService.configure(appContext, enabled))
            }
            "floatingBubbleDisable" -> {
                FloatingChatBubbleService.disable(appContext)
                result.success(FloatingChatBubbleService.status(appContext))
            }
            "floatingBubblePermission" -> {
                val intent = Intent(
                    Settings.ACTION_MANAGE_OVERLAY_PERMISSION,
                    Uri.parse("package:${appContext.packageName}"),
                ).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                appContext.startActivity(intent)
                result.success(true)
            }
            "sendLocalNotificationScenario" -> {
                val scenario = call.argument<String>("scenario").orEmpty()
                val now = System.currentTimeMillis().toString()
                val common = mapOf(
                    "sound" to "true",
                    "deepLink" to "amitia://settings/notifications",
                )
                if (scenario == "task") {
                    if (!NotificationManagerCompat.from(appContext).areNotificationsEnabled()) {
                        result.error("NOTIFICATIONS_DISABLED", "请先允许系统通知", null)
                    } else {
                        startTaskScenario(now, common)
                        result.success(true)
                    }
                } else {
                    val data = when (scenario) {
                        "message" -> common + mapOf(
                            "deepLink" to "amitia://chat/local-scenario",
                            "type" to "message.received",
                            "conversationId" to "local-scenario",
                            "characterId" to "amitia-scenario",
                            "title" to "Amitia Test",
                            "body" to "Local notification scenario: a new message arrived.",
                        )
                        "reminder" -> common + mapOf(
                            "type" to "reminder.triggered",
                            "messageId" to now,
                            "title" to "Amitia reminder test",
                            "body" to "Local reminder notification, independent of cloud push.",
                        )
                        else -> null
                    }
                    if (data == null) {
                        result.error("UNKNOWN_SCENARIO", "Unsupported local scenario", null)
                    } else {
                        NotificationRenderer.handleRemoteMessage(appContext, data)
                        result.success(true)
                    }
                }
            }
            "openSettings" -> {
                val intent = Intent(Settings.ACTION_APP_NOTIFICATION_SETTINGS)
                    .putExtra(Settings.EXTRA_APP_PACKAGE, appContext.packageName)
                    .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                appContext.startActivity(intent)
                result.success(true)
            }
            else -> result.notImplemented()
        }
    }

    private fun startTaskScenario(now: String, common: Map<String, String>) {
        scenarioRunnable?.let { scenarioHandler.removeCallbacks(it) }
        scenarioRunId?.let { previous ->
            NotificationRenderer.handleRemoteMessage(
                appContext,
                mapOf("type" to "run.dismiss", "runId" to previous),
            )
        }
        val runId = "local-scenario-$now"
        scenarioRunId = runId
        val application = appContext.applicationContext
        var step = 0
        val tick = object : Runnable {
            override fun run() {
                if (scenarioRunId != runId) return
                val finished = step >= 6
                val data = common + mapOf(
                    "type" to if (finished) "run.completed" else if (step == 0) "run.started" else "run.updated",
                    "runId" to runId,
                    "revision" to (step + 1).toString(),
                    "phase" to if (finished) "completed" else "running",
                    "title" to "Amitia Agent 模拟任务",
                    "summary" to if (finished) "模拟任务已完成" else "模拟执行中：第 $step / 6 步",
                    "currentStep" to step.toString(),
                    "totalSteps" to "6",
                    "progress" to (step.toDouble() / 6).toString(),
                    "sound" to "false",
                )
                NotificationRenderer.handleRemoteMessage(application, data)
                if (finished) {
                    scenarioRunnable = null
                    scenarioRunId = null
                } else {
                    step += 1
                    scenarioHandler.postDelayed(this, 4_000L)
                }
            }
        }
        scenarioRunnable = tick
        scenarioHandler.post(tick)
    }

    private fun requestPermission(result: MethodChannel.Result) {
        val activity = MainActivity.currentActivity()
        if (activity == null) {
            val enabled = NotificationManagerCompat.from(appContext).areNotificationsEnabled()
            if (enabled) {
                appContext.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
                    .edit()
                    .putBoolean(VENDOR_PUSH_OPT_IN, true)
                    .apply()
                VendorPushBootstrap.refresh(appContext)
            }
            result.success(enabled)
            return
        }
        CoroutineScope(Dispatchers.Main).launch {
            val granted = activity.requestNotificationPostPermission()
            if (granted) {
                appContext.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
                    .edit()
                    .putBoolean(VENDOR_PUSH_OPT_IN, true)
                    .apply()
                VendorPushBootstrap.refresh(appContext)
            }
            result.success(granted)
        }
    }

    private fun setPushEnabled(enabled: Boolean) {
        val prefs = appContext.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        if (!enabled) {
            prefs.edit().putBoolean(VENDOR_PUSH_OPT_IN, false).apply()
            VendorPushBootstrap.disable(appContext)
            return
        }
        if (NotificationManagerCompat.from(appContext).areNotificationsEnabled()) {
            prefs.edit().putBoolean(VENDOR_PUSH_OPT_IN, true).apply()
            VendorPushBootstrap.refresh(appContext)
        }
    }

    private fun vendorPushOptedIn(): Boolean =
        appContext.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            .getBoolean(VENDOR_PUSH_OPT_IN, false)

    private fun capabilities(configured: Boolean, token: String?): Map<String, Any?> {
        val vendorTokens = VendorPushBootstrap.currentTokens(appContext)
        val hint = VendorPushBootstrap.manufacturerHint()
        val preferred = if (hint.isNotBlank() && vendorTokens[hint]?.isNotBlank() == true) {
            hint
        } else {
            "fcm"
        }
        return mapOf(
            "platform" to "android",
            "provider" to preferred,
            "pushConfigured" to (configured || vendorTokens.isNotEmpty()),
            "pushToken" to token,
            "notificationsEnabled" to NotificationManagerCompat.from(appContext).areNotificationsEnabled(),
            "progressStyleSupported" to (Build.VERSION.SDK_INT >= 36),
            "liveActivitySupported" to false,
            "dynamicIslandSupported" to (
                AgentTaskIslandRouting.choose(appContext) ==
                    AgentTaskIslandRouting.Route.XIAOMI_SUPER_ISLAND ||
                AgentTaskIslandRouting.choose(appContext) ==
                    AgentTaskIslandRouting.Route.VIVO_ORIGIN_ISLAND
            ),
            "islandProvider" to AgentTaskIslandRouting.providerName(
                AgentTaskIslandRouting.choose(appContext),
            ),
            "xiaomiFocusPermissionGranted" to XiaomiIslandTaskCompat.focusPermission(appContext),
            "communicationNotificationSupported" to true,
            "vendorTokens" to vendorTokens,
            "invalidatedProviders" to VendorPushBootstrap.invalidatedProviders(appContext),
            "vendorSdkAvailability" to VendorPushBootstrap.sdkAvailability(),
            "nativeDataProviders" to VendorPushBootstrap.nativeDataProviders(),
            "vendorHint" to hint,
        )
    }

    private fun consumePendingInteraction(): Map<String, Any?>? {
        val prefs = appContext.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        val raw = prefs.getString(PENDING, null)?.takeIf { it.isNotBlank() } ?: return null
        prefs.edit().remove(PENDING).apply()
        val json = org.json.JSONObject(raw)
        return mapOf(
            "deepLink" to json.optString("deepLink").takeIf { it.isNotBlank() },
            "reply" to json.optString("reply").takeIf { it.isNotBlank() },
            "conversationId" to json.optString("conversationId").takeIf { it.isNotBlank() },
            "source" to json.optString("source").takeIf { it.isNotBlank() },
        )
    }
}
