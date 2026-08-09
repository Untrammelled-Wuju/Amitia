package com.amitia.amitia_app.runtime.install

import java.io.File

internal object PathValidator {

    private val VALID_VERSION_PATTERN = Regex("^[a-zA-Z0-9._+\\-]+$")

    fun isValidRuntimeVersion(version: String): Boolean {
        if (version.isBlank()) return false
        if (version.contains("..")) return false
        if (version.startsWith("/")) return false
        if (version.startsWith("\\")) return false
        if (version.contains(":")) return false
        if (version.contains("/")) return false
        if (version.contains("\\")) return false
        if (!VALID_VERSION_PATTERN.matches(version)) return false
        if (!version.any { it.isDigit() }) return false
        return true
    }

    fun isWithin(child: File, parent: File): Boolean {
        return try {
            val childCanonical = child.canonicalPath
            val parentCanonical = parent.canonicalPath
            childCanonical == parentCanonical || childCanonical.startsWith(
                parentCanonical + File.separator
            )
        } catch (_: Exception) {
            val childAbsolute = child.absolutePath
            val parentAbsolute = parent.absolutePath
            childAbsolute == parentAbsolute || childAbsolute.startsWith(
                parentAbsolute + File.separator
            )
        }
    }

    fun isExternalPath(path: File): Boolean {
        return try {
            val canonical = path.canonicalPath
            !canonical.contains("/data/") && !canonical.contains("\\data\\")
        } catch (_: Exception) {
            false
        }
    }

    fun requireWithin(child: File, parent: File, pathName: String = "path") {
        require(isWithin(child, parent)) {
            "$pathName must be within parent directory: child=${child.absolutePath} parent=${parent.absolutePath}"
        }
    }

    fun requireValidVersion(version: String) {
        require(isValidRuntimeVersion(version)) {
            "invalid runtime version: $version"
        }
    }

    fun isAbsolutePathSafe(path: String): Boolean {
        if (path.startsWith("/sdcard")) return false
        if (path.startsWith("/storage/emulated")) return false
        if (path.contains("..")) return false
        return true
    }
}
