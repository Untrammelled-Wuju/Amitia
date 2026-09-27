package com.amitia.amitia_app.nativeprovider.accessibility

import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.os.Build
import android.provider.Settings

internal class AccessibilitySettingsLauncher(private val context: Context) {

    fun openSettings(): Boolean {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O && openServiceDetails()) {
            return true
        }
        val intent = Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS)
        intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        return try {
            context.startActivity(intent)
            true
        } catch (_: Exception) {
            false
        }
    }

    private fun openServiceDetails(): Boolean {
        val component = ComponentName(
            context.packageName,
            AmitiaAccessibilityService::class.java.name,
        )
        val intent = Intent(ACTION_ACCESSIBILITY_DETAILS_SETTINGS)
            .putExtra(Intent.EXTRA_COMPONENT_NAME, component)
            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        return try {
            context.startActivity(intent)
            true
        } catch (_: Exception) {
            false
        }
    }

    companion object {
        private const val ACTION_ACCESSIBILITY_DETAILS_SETTINGS =
            "android.settings.ACCESSIBILITY_DETAILS_SETTINGS"
    }
}
