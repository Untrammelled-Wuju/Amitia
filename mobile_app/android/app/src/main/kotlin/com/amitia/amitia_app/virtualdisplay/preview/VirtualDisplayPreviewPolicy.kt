package com.amitia.amitia_app.virtualdisplay.preview

internal object VirtualDisplayPreviewPolicy {
    private val directOperations = setOf(
        "virtual_display.create",
        "virtual_display.launch",
        "virtual_display.capture",
        "virtual_display.tap",
        "virtual_display.swipe",
        "virtual_display.key",
        "virtual_display.text",
        "virtual_display.clear_text",
        "virtual_display.ui_tree",
        "virtual_display.resize",
    )

    fun shouldObserveBefore(operation: String): Boolean {
        if (operation == "virtual_display.create" || operation == "virtual_display.release") {
            return false
        }
        return isPreviewOperation(operation)
    }

    fun shouldObserveAfter(operation: String): Boolean {
        return operation == "virtual_display.create" || operation == "virtual_display.release"
    }

    fun isPreviewOperation(operation: String): Boolean {
        if (operation in directOperations) return true
        return operation.startsWith("interaction.") && operation != "interaction.status"
    }

    fun displayId(payload: Map<String, Any?>): Int {
        return (payload["displayId"] as? Number)?.toInt() ?: -1
    }
}
