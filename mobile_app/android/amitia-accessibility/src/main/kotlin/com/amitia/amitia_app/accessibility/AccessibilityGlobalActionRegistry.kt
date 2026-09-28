package com.amitia.amitia_app.accessibility

import android.accessibilityservice.AccessibilityService

object AccessibilityGlobalActionRegistry {

    data class Definition(
        val name: String,
        val actionId: Int,
    )

    val definitions: List<Definition> = listOf(
        Definition("back", AccessibilityService.GLOBAL_ACTION_BACK),
        Definition("home", AccessibilityService.GLOBAL_ACTION_HOME),
        Definition("recents", AccessibilityService.GLOBAL_ACTION_RECENTS),
        Definition("notifications", AccessibilityService.GLOBAL_ACTION_NOTIFICATIONS),
        Definition("quick_settings", AccessibilityService.GLOBAL_ACTION_QUICK_SETTINGS),
        Definition("power_dialog", AccessibilityService.GLOBAL_ACTION_POWER_DIALOG),
        Definition("toggle_split_screen", AccessibilityService.GLOBAL_ACTION_TOGGLE_SPLIT_SCREEN),
        Definition("lock_screen", AccessibilityService.GLOBAL_ACTION_LOCK_SCREEN),
        Definition("take_screenshot", AccessibilityService.GLOBAL_ACTION_TAKE_SCREENSHOT),
        Definition("keycode_headset_hook", AccessibilityService.GLOBAL_ACTION_KEYCODE_HEADSETHOOK),
        Definition("accessibility_shortcut", AccessibilityService.GLOBAL_ACTION_ACCESSIBILITY_SHORTCUT),
        Definition("accessibility_all_apps", AccessibilityService.GLOBAL_ACTION_ACCESSIBILITY_ALL_APPS),
        Definition("accessibility_button", AccessibilityService.GLOBAL_ACTION_ACCESSIBILITY_BUTTON),
        Definition("accessibility_button_chooser", AccessibilityService.GLOBAL_ACTION_ACCESSIBILITY_BUTTON_CHOOSER),
        Definition("dismiss_notification_shade", AccessibilityService.GLOBAL_ACTION_DISMISS_NOTIFICATION_SHADE),
        Definition("dpad_up", AccessibilityService.GLOBAL_ACTION_DPAD_UP),
        Definition("dpad_down", AccessibilityService.GLOBAL_ACTION_DPAD_DOWN),
        Definition("dpad_left", AccessibilityService.GLOBAL_ACTION_DPAD_LEFT),
        Definition("dpad_right", AccessibilityService.GLOBAL_ACTION_DPAD_RIGHT),
        Definition("dpad_center", AccessibilityService.GLOBAL_ACTION_DPAD_CENTER),
        Definition("media_play_pause", AccessibilityService.GLOBAL_ACTION_MEDIA_PLAY_PAUSE),
        Definition("menu", AccessibilityService.GLOBAL_ACTION_MENU),
    )

    fun resolve(name: String): Definition? =
        definitions.firstOrNull { it.name == name.trim().lowercase() }
}
