package com.amitia.amitia_app.notifications

import android.content.Context
import android.os.Build

/**
 * One route decision shared by execution notifications and settings diagnostics.
 * A manufacturer's island API has priority only when this app can request it.
 * On devices without such an integration we use Android 16 Live Updates or,
 * on earlier releases, a standard ongoing progress notification.
 */
internal object AgentTaskIslandRouting {
    enum class Route {
        XIAOMI_SUPER_ISLAND,
        VIVO_ORIGIN_ISLAND,
        ANDROID_LIVE_UPDATE,
        STANDARD_PROGRESS,
    }

    fun choose(context: Context): Route = when {
        XiaomiIslandTaskCompat.supported(context) &&
            XiaomiIslandTaskCompat.focusPermission(context) -> Route.XIAOMI_SUPER_ISLAND
        VivoOriginIslandTaskCompat.eligible() -> Route.VIVO_ORIGIN_ISLAND
        Build.VERSION.SDK_INT >= 36 -> Route.ANDROID_LIVE_UPDATE
        else -> Route.STANDARD_PROGRESS
    }

    fun shouldRequestLiveUpdate(): Boolean =
        // Vendor payload is always attached first. However, scene admission is
        // not reliably queryable for any manufacturer, including Xiaomi:
        // "canShowFocus" does NOT mean this business scene was approved.
        // Use the same notification's Android 16 promotion as a safe fallback.
        Build.VERSION.SDK_INT >= 36

    fun providerName(route: Route): String = when (route) {
        Route.XIAOMI_SUPER_ISLAND -> "xiaomi_super_island"
        Route.VIVO_ORIGIN_ISLAND -> "vivo_origin_island"
        Route.ANDROID_LIVE_UPDATE -> "android_live_update"
        Route.STANDARD_PROGRESS -> "standard_progress"
    }
}
