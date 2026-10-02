package com.amitia.amitia_app.nativeprovider.display

import android.app.Activity
import android.content.Context
import android.view.WindowManager

internal object ScreenAwakeController {
    private const val PREFERENCES = "amitia.display.preferences"
    private const val KEY = "keepScreenOn"

    fun enabled(context: Context): Boolean =
        context.getSharedPreferences(PREFERENCES, Context.MODE_PRIVATE).getBoolean(KEY, false)

    fun setEnabled(context: Context, enabled: Boolean): Boolean =
        context.getSharedPreferences(PREFERENCES, Context.MODE_PRIVATE).edit().putBoolean(KEY, enabled).commit()

    fun apply(activity: Activity, foreground: Boolean) {
        if (foreground && enabled(activity)) {
            activity.window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
        } else {
            activity.window.clearFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
        }
    }
}
