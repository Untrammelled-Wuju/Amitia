package com.amitia.amitia_app.nativeprovider.uitree

import android.accessibilityservice.AccessibilityService
import android.os.Build
import android.view.accessibility.AccessibilityNodeInfo

/**
 * Short-lived semantic references for Accessibility nodes.
 *
 * References carry the snapshot generation, window id and child-index path.
 * They are resolved against a fresh Accessibility tree at action time instead
 * of retaining AccessibilityNodeInfo instances across window/activity changes.
 */
internal object AccessibilityNodeReferenceRegistry {
    @Volatile
    private var currentGeneration: Long = 0L

    fun beginSnapshot(generation: Long) {
        currentGeneration = generation
    }

    fun invalidateAll() {
        currentGeneration = Long.MIN_VALUE
    }

    fun build(generation: Long, nativeWindowId: Int, childPath: List<Int>): String {
        val path = if (childPath.isEmpty()) "root" else childPath.joinToString(".")
        return "acc:$generation:$nativeWindowId:$path"
    }

    fun resolve(service: AccessibilityService, nativeRef: String): AccessibilityNodeInfo? {
        val parsed = parse(nativeRef) ?: return null
        if (parsed.generation != currentGeneration) return null

        var node: AccessibilityNodeInfo = if (parsed.windowId == ACTIVE_WINDOW_SENTINEL) {
            service.rootInActiveWindow ?: return null
        } else {
            val window = allWindows(service).firstOrNull { it.id == parsed.windowId } ?: return null
            window.root ?: return null
        }

        for (index in parsed.path) {
            if (index < 0 || index >= node.childCount) return null
            node = node.getChild(index) ?: return null
        }
        return node
    }

    private fun allWindows(service: AccessibilityService): List<android.view.accessibility.AccessibilityWindowInfo> {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.R) {
            return service.windows.orEmpty()
        }
        return runCatching {
            val windows = service.windowsOnAllDisplays
            val result = mutableListOf<android.view.accessibility.AccessibilityWindowInfo>()
            for (index in 0 until windows.size()) {
                windows.valueAt(index)?.let { result.addAll(it) }
            }
            if (result.isNotEmpty()) result else service.windows.orEmpty()
        }.getOrElse { service.windows.orEmpty() }
    }

    private fun parse(value: String): ParsedReference? {
        val parts = value.split(':', limit = 4)
        if (parts.size != 4 || parts[0] != "acc") return null
        val generation = parts[1].toLongOrNull() ?: return null
        val windowId = parts[2].toIntOrNull() ?: return null
        val path = when (parts[3]) {
            "", "root" -> emptyList()
            else -> parts[3].split('.').map { it.toIntOrNull() ?: return null }
        }
        return ParsedReference(generation, windowId, path)
    }

    private data class ParsedReference(
        val generation: Long,
        val windowId: Int,
        val path: List<Int>,
    )

    const val ACTIVE_WINDOW_SENTINEL: Int = -1
}
