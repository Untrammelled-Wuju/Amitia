package com.amitia.amitia_app.nativeprovider.accessibility

import android.content.Context
import android.content.Intent
import android.provider.Settings

internal class AccessibilitySettingsLauncher(private val context: Context) {

    fun canOpenSettings(): Boolean {
        val intent = Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS)
        return intent.resolveActivity(context.packageManager) != null
    }

    fun openSettings(): Boolean {
        val intent = Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS)
        intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        return try {
            context.startActivity(intent)
            true
        } catch (_: Exception) {
            false
        }
    }
}
