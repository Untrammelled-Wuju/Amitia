package com.amitia.amitia_app.accessibility

import android.os.Bundle
import android.os.Build
import android.view.accessibility.AccessibilityNodeInfo

object AccessibilityNodeActionExecutor {

    data class Result(
        val performed: Boolean,
        val message: String = "",
    )

    val supportedActions: Set<String> = setOf(
        "click",
        "long_click",
        "focus",
        "clear_focus",
        "select",
        "clear_selection",
        "set_text",
        "clear_text",
        "set_selection",
        "copy",
        "cut",
        "paste",
        "scroll_forward",
        "scroll_backward",
        "scroll_up",
        "scroll_down",
        "scroll_left",
        "scroll_right",
        "scroll_to_position",
        "expand",
        "collapse",
        "dismiss",
        "show_on_screen",
        "context_click",
        "accessibility_focus",
        "clear_accessibility_focus",
        "show_tooltip",
        "hide_tooltip",
        "ime_enter",
        "press_and_hold",
        "next_at_movement_granularity",
        "previous_at_movement_granularity",
        "next_html_element",
        "previous_html_element",
        "set_progress",
    )

    fun perform(
        node: AccessibilityNodeInfo,
        action: String,
        args: Map<String, Any?>,
    ): Result {
        val normalized = action.trim().lowercase()
        if (normalized !in supportedActions) {
            return Result(false, "unsupported node action: $normalized")
        }
        return try {
            val performed = when (normalized) {
                "click" -> node.performAction(AccessibilityNodeInfo.ACTION_CLICK)
                "long_click" -> node.performAction(AccessibilityNodeInfo.ACTION_LONG_CLICK)
                "focus" -> node.performAction(AccessibilityNodeInfo.ACTION_FOCUS)
                "clear_focus" -> node.performAction(AccessibilityNodeInfo.ACTION_CLEAR_FOCUS)
                "select" -> node.performAction(AccessibilityNodeInfo.ACTION_SELECT)
                "clear_selection" -> node.performAction(AccessibilityNodeInfo.ACTION_CLEAR_SELECTION)
                "set_text" -> node.performAction(
                    AccessibilityNodeInfo.ACTION_SET_TEXT,
                    Bundle().apply {
                        putCharSequence(
                            AccessibilityNodeInfo.ACTION_ARGUMENT_SET_TEXT_CHARSEQUENCE,
                            stringArg(args, "text").orEmpty(),
                        )
                    },
                )
                "clear_text" -> node.performAction(
                    AccessibilityNodeInfo.ACTION_SET_TEXT,
                    Bundle().apply {
                        putCharSequence(
                            AccessibilityNodeInfo.ACTION_ARGUMENT_SET_TEXT_CHARSEQUENCE,
                            "",
                        )
                    },
                )
                "set_selection" -> node.performAction(
                    AccessibilityNodeInfo.ACTION_SET_SELECTION,
                    Bundle().apply {
                        putInt(
                            AccessibilityNodeInfo.ACTION_ARGUMENT_SELECTION_START_INT,
                            intArg(args, "start", 0),
                        )
                        putInt(
                            AccessibilityNodeInfo.ACTION_ARGUMENT_SELECTION_END_INT,
                            intArg(args, "end", 0),
                        )
                    },
                )
                "copy" -> node.performAction(AccessibilityNodeInfo.ACTION_COPY)
                "cut" -> node.performAction(AccessibilityNodeInfo.ACTION_CUT)
                "paste" -> node.performAction(AccessibilityNodeInfo.ACTION_PASTE)
                "scroll_forward" -> node.performAction(AccessibilityNodeInfo.ACTION_SCROLL_FORWARD)
                "scroll_backward" -> node.performAction(AccessibilityNodeInfo.ACTION_SCROLL_BACKWARD)
                "scroll_up" -> node.performAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_SCROLL_UP.id)
                "scroll_down" -> node.performAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_SCROLL_DOWN.id)
                "scroll_left" -> node.performAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_SCROLL_LEFT.id)
                "scroll_right" -> node.performAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_SCROLL_RIGHT.id)
                "scroll_to_position" -> node.performAction(
                    AccessibilityNodeInfo.AccessibilityAction.ACTION_SCROLL_TO_POSITION.id,
                    Bundle().apply {
                        putInt(
                            AccessibilityNodeInfo.ACTION_ARGUMENT_ROW_INT,
                            intArg(args, "row", 0),
                        )
                        putInt(
                            AccessibilityNodeInfo.ACTION_ARGUMENT_COLUMN_INT,
                            intArg(args, "column", 0),
                        )
                    },
                )
                "expand" -> node.performAction(AccessibilityNodeInfo.ACTION_EXPAND)
                "collapse" -> node.performAction(AccessibilityNodeInfo.ACTION_COLLAPSE)
                "dismiss" -> node.performAction(AccessibilityNodeInfo.ACTION_DISMISS)
                "show_on_screen" -> node.performAction(
                    AccessibilityNodeInfo.AccessibilityAction.ACTION_SHOW_ON_SCREEN.id,
                )
                "context_click" -> node.performAction(
                    AccessibilityNodeInfo.AccessibilityAction.ACTION_CONTEXT_CLICK.id,
                )
                "accessibility_focus" -> node.performAction(AccessibilityNodeInfo.ACTION_ACCESSIBILITY_FOCUS)
                "clear_accessibility_focus" -> node.performAction(AccessibilityNodeInfo.ACTION_CLEAR_ACCESSIBILITY_FOCUS)
                "show_tooltip" -> node.performAction(
                    AccessibilityNodeInfo.AccessibilityAction.ACTION_SHOW_TOOLTIP.id,
                )
                "hide_tooltip" -> node.performAction(
                    AccessibilityNodeInfo.AccessibilityAction.ACTION_HIDE_TOOLTIP.id,
                )
                "ime_enter" -> node.performAction(
                    AccessibilityNodeInfo.AccessibilityAction.ACTION_IME_ENTER.id,
                )
                "press_and_hold" -> node.performAction(
                    AccessibilityNodeInfo.AccessibilityAction.ACTION_PRESS_AND_HOLD.id,
                    Bundle().apply {
                        putInt(
                            AccessibilityNodeInfo.ACTION_ARGUMENT_PRESS_AND_HOLD_DURATION_MILLIS_INT,
                            intArg(args, "durationMs", 600),
                        )
                    },
                )
                "next_at_movement_granularity" -> node.performAction(
                    AccessibilityNodeInfo.ACTION_NEXT_AT_MOVEMENT_GRANULARITY,
                    movementBundle(args),
                )
                "previous_at_movement_granularity" -> node.performAction(
                    AccessibilityNodeInfo.ACTION_PREVIOUS_AT_MOVEMENT_GRANULARITY,
                    movementBundle(args),
                )
                "next_html_element" -> node.performAction(
                    AccessibilityNodeInfo.ACTION_NEXT_HTML_ELEMENT,
                    Bundle().apply {
                        putString(
                            AccessibilityNodeInfo.ACTION_ARGUMENT_HTML_ELEMENT_STRING,
                            stringArg(args, "element"),
                        )
                    },
                )
                "previous_html_element" -> node.performAction(
                    AccessibilityNodeInfo.ACTION_PREVIOUS_HTML_ELEMENT,
                    Bundle().apply {
                        putString(
                            AccessibilityNodeInfo.ACTION_ARGUMENT_HTML_ELEMENT_STRING,
                            stringArg(args, "element"),
                        )
                    },
                )
                "set_progress" -> node.performAction(
                    AccessibilityNodeInfo.AccessibilityAction.ACTION_SET_PROGRESS.id,
                    Bundle().apply {
                        putFloat(
                            AccessibilityNodeInfo.ACTION_ARGUMENT_PROGRESS_VALUE,
                            floatArg(args, "value", 0f),
                        )
                    },
                )
                else -> false
            }
            Result(performed, if (performed) "" else "accessibility node action returned false")
        } catch (error: Throwable) {
            Result(false, error.message ?: error.javaClass.simpleName)
        }
    }

    fun actionName(action: Int): String? = when (action) {
        AccessibilityNodeInfo.ACTION_CLICK -> "click"
        AccessibilityNodeInfo.ACTION_LONG_CLICK -> "long_click"
        AccessibilityNodeInfo.ACTION_FOCUS -> "focus"
        AccessibilityNodeInfo.ACTION_CLEAR_FOCUS -> "clear_focus"
        AccessibilityNodeInfo.ACTION_SELECT -> "select"
        AccessibilityNodeInfo.ACTION_CLEAR_SELECTION -> "clear_selection"
        AccessibilityNodeInfo.ACTION_SET_TEXT -> "set_text"
        AccessibilityNodeInfo.ACTION_SET_SELECTION -> "set_selection"
        AccessibilityNodeInfo.ACTION_COPY -> "copy"
        AccessibilityNodeInfo.ACTION_CUT -> "cut"
        AccessibilityNodeInfo.ACTION_PASTE -> "paste"
        AccessibilityNodeInfo.ACTION_SCROLL_FORWARD -> "scroll_forward"
        AccessibilityNodeInfo.ACTION_SCROLL_BACKWARD -> "scroll_backward"
        AccessibilityNodeInfo.AccessibilityAction.ACTION_SCROLL_UP.id -> "scroll_up"
        AccessibilityNodeInfo.AccessibilityAction.ACTION_SCROLL_DOWN.id -> "scroll_down"
        AccessibilityNodeInfo.AccessibilityAction.ACTION_SCROLL_LEFT.id -> "scroll_left"
        AccessibilityNodeInfo.AccessibilityAction.ACTION_SCROLL_RIGHT.id -> "scroll_right"
        AccessibilityNodeInfo.AccessibilityAction.ACTION_SCROLL_TO_POSITION.id -> "scroll_to_position"
        AccessibilityNodeInfo.ACTION_EXPAND -> "expand"
        AccessibilityNodeInfo.ACTION_COLLAPSE -> "collapse"
        AccessibilityNodeInfo.ACTION_DISMISS -> "dismiss"
        AccessibilityNodeInfo.AccessibilityAction.ACTION_SHOW_ON_SCREEN.id -> "show_on_screen"
        AccessibilityNodeInfo.AccessibilityAction.ACTION_CONTEXT_CLICK.id -> "context_click"
        AccessibilityNodeInfo.ACTION_ACCESSIBILITY_FOCUS -> "accessibility_focus"
        AccessibilityNodeInfo.ACTION_CLEAR_ACCESSIBILITY_FOCUS -> "clear_accessibility_focus"
        AccessibilityNodeInfo.ACTION_NEXT_AT_MOVEMENT_GRANULARITY -> "next_at_movement_granularity"
        AccessibilityNodeInfo.ACTION_PREVIOUS_AT_MOVEMENT_GRANULARITY -> "previous_at_movement_granularity"
        AccessibilityNodeInfo.ACTION_NEXT_HTML_ELEMENT -> "next_html_element"
        AccessibilityNodeInfo.ACTION_PREVIOUS_HTML_ELEMENT -> "previous_html_element"
        AccessibilityNodeInfo.AccessibilityAction.ACTION_SET_PROGRESS.id -> "set_progress"
        else -> when {
            Build.VERSION.SDK_INT >= Build.VERSION_CODES.O &&
                action == AccessibilityNodeInfo.AccessibilityAction.ACTION_PRESS_AND_HOLD.id -> "press_and_hold"
            Build.VERSION.SDK_INT >= Build.VERSION_CODES.P &&
                action == AccessibilityNodeInfo.AccessibilityAction.ACTION_SHOW_TOOLTIP.id -> "show_tooltip"
            Build.VERSION.SDK_INT >= Build.VERSION_CODES.P &&
                action == AccessibilityNodeInfo.AccessibilityAction.ACTION_HIDE_TOOLTIP.id -> "hide_tooltip"
            Build.VERSION.SDK_INT >= Build.VERSION_CODES.R &&
                action == AccessibilityNodeInfo.AccessibilityAction.ACTION_IME_ENTER.id -> "ime_enter"
            else -> null
        }
    }

    private fun movementBundle(args: Map<String, Any?>): Bundle = Bundle().apply {
        putInt(
            AccessibilityNodeInfo.ACTION_ARGUMENT_MOVEMENT_GRANULARITY_INT,
            movementGranularity(args["granularity"] ?: args["movementGranularity"]),
        )
        putBoolean(
            AccessibilityNodeInfo.ACTION_ARGUMENT_EXTEND_SELECTION_BOOLEAN,
            boolArg(args, "extendSelection", false),
        )
    }

    private fun movementGranularity(value: Any?): Int = when (value?.toString()?.lowercase()) {
        "character" -> AccessibilityNodeInfo.MOVEMENT_GRANULARITY_CHARACTER
        "word" -> AccessibilityNodeInfo.MOVEMENT_GRANULARITY_WORD
        "line" -> AccessibilityNodeInfo.MOVEMENT_GRANULARITY_LINE
        "paragraph" -> AccessibilityNodeInfo.MOVEMENT_GRANULARITY_PARAGRAPH
        "page" -> AccessibilityNodeInfo.MOVEMENT_GRANULARITY_PAGE
        else -> (value as? Number)?.toInt() ?: AccessibilityNodeInfo.MOVEMENT_GRANULARITY_CHARACTER
    }

    private fun stringArg(args: Map<String, Any?>, key: String): String? =
        args[key]?.toString()

    private fun intArg(args: Map<String, Any?>, key: String, fallback: Int): Int =
        (args[key] as? Number)?.toInt() ?: fallback

    private fun floatArg(args: Map<String, Any?>, key: String, fallback: Float): Float =
        (args[key] as? Number)?.toFloat() ?: fallback

    private fun boolArg(args: Map<String, Any?>, key: String, fallback: Boolean): Boolean =
        args[key] as? Boolean ?: fallback
}
