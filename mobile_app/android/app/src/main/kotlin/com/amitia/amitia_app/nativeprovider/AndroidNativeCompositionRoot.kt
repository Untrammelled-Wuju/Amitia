package com.amitia.amitia_app.nativeprovider

import android.content.Context
import com.amitia.amitia_app.nativeprovider.accessibility.AccessibilityNativeHandler
import com.amitia.amitia_app.nativeprovider.accessibility.AccessibilityNativeHandlerAdapter
import com.amitia.amitia_app.nativeprovider.camera.CameraNativeHandler
import com.amitia.amitia_app.nativeprovider.clipboard.ClipboardNativeHandler
import com.amitia.amitia_app.nativeprovider.clipboard.ClipboardNativeHandlerAdapter
import com.amitia.amitia_app.nativeprovider.display.DisplayNativeHandler
import com.amitia.amitia_app.nativeprovider.externalautomation.ExternalAutomationNativeHandler
import com.amitia.amitia_app.nativeprovider.interaction.InteractionNativeHandler
import com.amitia.amitia_app.nativeprovider.notification.NotificationNativeHandler
import com.amitia.amitia_app.nativeprovider.notification.NotificationNativeHandlerAdapter
import com.amitia.amitia_app.nativeprovider.overlay.OverlayNativeHandler
import com.amitia.amitia_app.nativeprovider.root.RootNativeHandler
import com.amitia.amitia_app.nativeprovider.share.ShareNativeHandler
import com.amitia.amitia_app.nativeprovider.share.ShareNativeHandlerAdapter
import com.amitia.amitia_app.nativeprovider.uitree.UITreeNativeHandler
import com.amitia.amitia_app.nativeprovider.virtualdisplay.VirtualDisplayNativeHandler
import kotlinx.coroutines.runBlocking

internal object AndroidNativeCompositionRoot {

    @Volatile
    private var initialized = false

    fun initialize(context: Context) {
        if (initialized) return
        synchronized(this) {
            if (initialized) return
            val appContext = context.applicationContext
            val host = AndroidNativeHost.shared(appContext)

            val handlers = buildHandlers(appContext)
            runBlocking {
                handlers.forEach { handler ->
                    host.registerHandler(handler)
                }
            }

            initialized = true
        }
    }

    private fun buildHandlers(context: Context): List<AndroidNativeOperationHandler> {
        return listOf(
            buildAccessibilityHandler(context),
            buildClipboardHandler(context),
            buildShareHandler(context),
            buildNotificationHandler(context),
            RootNativeHandler(context),
            UITreeNativeHandler(context),
            InteractionNativeHandler(context),
            DisplayNativeHandler(context),
            VirtualDisplayNativeHandler(context),
            CameraNativeHandler(context),
            OverlayNativeHandler(context),
            ExternalAutomationNativeHandler(context),
        )
    }

    private fun buildAccessibilityHandler(context: Context): AndroidNativeOperationHandler {
        val handler = AccessibilityNativeHandler(context)
        return AccessibilityNativeHandlerAdapter(handler)
    }

    private fun buildClipboardHandler(context: Context): AndroidNativeOperationHandler {
        val handler = ClipboardNativeHandler(context)
        return ClipboardNativeHandlerAdapter(handler)
    }

    private fun buildShareHandler(context: Context): AndroidNativeOperationHandler {
        val handler = ShareNativeHandler(context)
        return ShareNativeHandlerAdapter(handler)
    }

    private fun buildNotificationHandler(context: Context): AndroidNativeOperationHandler {
        val handler = NotificationNativeHandler(context)
        return NotificationNativeHandlerAdapter(handler)
    }
}
