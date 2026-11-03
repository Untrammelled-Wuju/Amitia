package com.amitia.amitia_app.accessibility

import android.app.Service
import android.content.Intent
import android.content.pm.PackageManager
import android.os.IBinder
import android.os.Binder
import android.os.Process
import org.json.JSONObject

class RemoteBinderService : Service() {

    private val binder = object : IAmitiaAccessibilityProvider.Stub() {
        override fun status(): String = trustedCall {
            UIAccessibilityService.current()?.statusJson()
                ?: JSONObject()
                    .put("connected", false)
                    .put("enabledInSettings", false)
                    .put("canRetrieveWindowContent", false)
                    .put("canPerformGestures", false)
                    .toString()
        }

        override fun snapshot(payloadJson: String): String = trustedCall {
            UIAccessibilityService.current()?.snapshotJson(payloadJson)
                ?: providerUnavailable().toString()
        }

        override fun performNodeAction(payloadJson: String): String = trustedCall {
            UIAccessibilityService.current()?.performNodeActionJson(payloadJson)
                ?: providerUnavailable().toString()
        }

        override fun performClick(x: Int, y: Int): String = trustedCall {
            UIAccessibilityService.current()?.performClick(x, y)
                ?: providerUnavailable().toString()
        }

        override fun performLongPress(x: Int, y: Int, durationMs: Long): String = trustedCall {
            UIAccessibilityService.current()?.performLongPress(x, y, durationMs)
                ?: providerUnavailable().toString()
        }

        override fun performSwipe(startX: Int, startY: Int, endX: Int, endY: Int, durationMs: Long): String = trustedCall {
            UIAccessibilityService.current()?.performSwipe(startX, startY, endX, endY, durationMs)
                ?: providerUnavailable().toString()
        }

        override fun performGesture(payloadJson: String): String = trustedCall {
            UIAccessibilityService.current()?.performGestureJson(payloadJson)
                ?: providerUnavailable().toString()
        }

        override fun takeScreenshot(displayId: Int): String = trustedCall {
            UIAccessibilityService.current()?.takeScreenshotJson(displayId)
                ?: providerUnavailable().toString()
        }

        override fun performGlobalAction(actionId: Int): String = trustedCall {
            UIAccessibilityService.current()?.performGlobalActionJson(actionId)
                ?: providerUnavailable().toString()
        }
    }

    override fun onBind(intent: Intent?): IBinder = binder

    private fun providerUnavailable(): JSONObject =
        JSONObject()
            .put("success", false)
            .put("connected", false)
            .put("message", "accessibility provider is not connected")

    private inline fun <T> trustedCall(block: () -> T): T {
        val callingUid = Binder.getCallingUid()
        if (
            callingUid != Process.myUid() &&
            packageManager.checkSignatures(callingUid, Process.myUid()) != PackageManager.SIGNATURE_MATCH
        ) {
            throw SecurityException(
                "accessibility provider rejected caller uid $callingUid: signature mismatch",
            )
        }
        return block()
    }
}
