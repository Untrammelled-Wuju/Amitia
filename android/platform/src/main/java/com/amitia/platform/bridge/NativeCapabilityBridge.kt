package com.amitia.platform.bridge

import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class NativeCapabilityBridge @Inject constructor(
    private val registry: CapabilityRegistry
) {

    fun listCapabilities(): List<String> = registry.listActions()

    fun hasCapability(action: String): Boolean = registry.hasAction(action)

    suspend fun execute(request: NativeActionRequest): NativeActionResult {
        return registry.execute(request)
    }

    suspend fun execute(action: String, params: Map<String, String> = emptyMap()): NativeActionResult {
        return execute(NativeActionRequest(action = action, params = params))
    }

    object Actions {
        const val FILE_PICK = "file_pick"
        const val IMAGE_PICK = "image_pick"
        const val CAMERA = "camera"
        const val MIC_RECORD = "mic_record"
        const val AUDIO_PLAY = "audio_play"
        const val NOTIFICATION = "notification"
        const val CLIPBOARD_READ = "clipboard_read"
        const val CLIPBOARD_WRITE = "clipboard_write"
        const val SHARE = "share"
        const val APP_DIR = "app_dir"
        const val SYSTEM_THEME = "system_theme"
        const val NETWORK_STATE = "network_state"
        const val BATTERY_STATE = "battery_state"
        const val FOREGROUND_STATE = "foreground_state"

        const val ACCESSIBILITY = "accessibility"
        const val MEDIA_PROJECTION = "media_projection"
        const val OVERLAY_WINDOW = "overlay_window"
        const val SHIZUKU = "shizuku"
        const val ROOT = "root"
        const val SCREEN_CAPTURE = "screen_capture"
        const val GESTURE = "gesture"
        const val COMPUTER_USE = "computer_use"
    }
}
