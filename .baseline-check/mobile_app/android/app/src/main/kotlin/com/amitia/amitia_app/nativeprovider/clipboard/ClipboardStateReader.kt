package com.amitia.amitia_app.nativeprovider.clipboard

import android.content.ClipboardManager
import android.content.Context
import android.os.Build

internal class ClipboardStateReader(context: Context) {

    private val appContext = context.applicationContext

    fun readState(): ClipboardCapabilityState {
        val clipboardManager = try {
            appContext.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
        } catch (e: Exception) {
            return ClipboardCapabilityState(
                supported = false,
                state = "host_unavailable",
                reason = "ClipboardManager unavailable: ${e.message}",
            )
        }

        val hasPrimaryClip = try {
            clipboardManager.hasPrimaryClip()
        } catch (e: Exception) {
            false
        }

        return ClipboardCapabilityState(
            supported = true,
            canWrite = true,
            canRead = false,
            appForeground = false,
            appHasInputFocus = false,
            readRequiresForeground = Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q,
            hasPrimaryClip = hasPrimaryClip,
            supportedMimeTypes = listOf("text/plain", "text/html"),
            maxTextBytes = MAX_TEXT_BYTES,
            state = "foreground_required",
            reason = "android native host focus state unknown",
        )
    }

    fun readStateWithFocus(hasFocus: Boolean, isForeground: Boolean): ClipboardCapabilityState {
        val clipboardManager = try {
            appContext.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
        } catch (e: Exception) {
            return ClipboardCapabilityState(
                supported = false,
                state = "host_unavailable",
                reason = "ClipboardManager unavailable: ${e.message}",
            )
        }

        val hasPrimaryClip = try {
            clipboardManager.hasPrimaryClip()
        } catch (e: Exception) {
            false
        }

        val readRequiresForeground = Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q
        val canRead = if (readRequiresForeground) {
            isForeground && hasFocus
        } else {
            true
        }

        val state = when {
            !canRead && readRequiresForeground && !isForeground -> "foreground_required"
            !canRead && readRequiresForeground && !hasFocus -> "focus_required"
            !hasPrimaryClip -> "empty"
            else -> "available"
        }

        return ClipboardCapabilityState(
            supported = true,
            canWrite = true,
            canRead = canRead,
            appForeground = isForeground,
            appHasInputFocus = hasFocus,
            readRequiresForeground = readRequiresForeground,
            hasPrimaryClip = hasPrimaryClip,
            supportedMimeTypes = listOf("text/plain", "text/html"),
            maxTextBytes = MAX_TEXT_BYTES,
            state = state,
            reason = "",
        )
    }

    companion object {
        const val MAX_TEXT_BYTES = 65536
    }
}
