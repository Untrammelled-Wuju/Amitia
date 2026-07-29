package com.amitia.runtime.extension.security

class SafePathValidator(
    private val maxPathLength: Int = 512,
    private val maxDirectoryDepth: Int = 32
) {
    companion object {
        private val WINDOWS_RESERVED = setOf(
            "con", "prn", "aux", "nul",
            "com1", "com2", "com3", "com4", "com5",
            "com6", "com7", "com8", "com9",
            "lpt1", "lpt2", "lpt3", "lpt4", "lpt5",
            "lpt6", "lpt7", "lpt8", "lpt9"
        )
    }

    data class PathCollision(
        val pathA: String,
        val pathB: String,
        val reason: String
    )

    fun normalizeArchivePath(input: String): String {
        if (!isUtf8Valid(input) || input.contains(0.toChar())) {
            throw NonUtf8PathException("non-UTF8 or null byte in path: $input")
        }

        if (input.length > maxPathLength) {
            throw PathTooLongException("path exceeds max length $maxPathLength: $input")
        }

        if (input.startsWith("/") || input.startsWith("\\") ||
            input.startsWith("//") || input.startsWith("\\\\")
        ) {
            throw AbsolutePathException("absolute path not allowed: $input")
        }

        if (input.length >= 2 && input[1] == ':') {
            throw AbsolutePathException("drive letter path not allowed: $input")
        }

        var normalized = input.replace("\\", "/")

        if (normalized.contains("../") || normalized.contains("..\\") ||
            normalized.endsWith("..") || normalized.startsWith("../")
        ) {
            throw PathTraversalException("path traversal detected: $input")
        }

        if (normalized.startsWith("./")) {
            normalized = normalized.substring(2)
        }

        val cleaned = cleanPath(normalized)
        if (cleaned == "." || cleaned == ".." || cleaned != normalized) {
            throw PathTraversalException("path traversal after clean: $input")
        }

        val parts = cleaned.split("/")
        if (parts.size > maxDirectoryDepth) {
            throw PathDepthExceededException("directory depth exceeds $maxDirectoryDepth: $input")
        }

        for (part in parts) {
            validatePathComponent(part)
        }

        return cleaned
    }

    private fun validatePathComponent(part: String) {
        if (part.isEmpty()) return

        val trimmed = part.trimEnd(' ', '.')
        if (trimmed != part) {
            throw WindowsReservedNameException("trailing space or dot in path component: $part")
        }

        var base = trimmed.lowercase()
        val dotIdx = base.lastIndexOf('.')
        if (dotIdx >= 0) {
            base = base.substring(0, dotIdx)
        }

        if (WINDOWS_RESERVED.contains(base)) {
            throw WindowsReservedNameException("windows reserved name: $part")
        }
    }

    fun resolveWithinRoot(root: String, normalized: String): String {
        val cleanRoot = cleanPath(root.replace("\\", "/"))
        val resolved = cleanPath("$cleanRoot/$normalized")

        if (!resolved.startsWith("$cleanRoot/") && resolved != cleanRoot) {
            throw PathTraversalException("path escapes root: $normalized")
        }

        return resolved
    }

    fun detectCollisions(paths: List<String>): List<PathCollision> {
        val seen = mutableMapOf<String, String>()
        val collisions = mutableListOf<PathCollision>()

        for (p in paths) {
            val key = p.lowercase()
            if (seen.containsKey(key)) {
                collisions.add(
                    PathCollision(
                        pathA = seen[key]!!,
                        pathB = p,
                        reason = "case_insensitive_collision"
                    )
                )
            } else {
                seen[key] = p
            }
        }

        return collisions
    }

    fun validatePath(p: String) {
        if (p.contains("..")) {
            throw PathTraversalException("path traversal detected: $p")
        }
        if (p.contains("//")) {
            throw InvalidStructureException("double slash in path: $p")
        }
    }

    private fun cleanPath(path: String): String {
        if (path.isEmpty()) return "."

        val segments = mutableListOf<String>()
        for (segment in path.split("/")) {
            when {
                segment.isEmpty() || segment == "." -> {
                    if (segments.isEmpty() && segment.isEmpty()) {
                        segments.add("")
                    }
                }
                segment == ".." -> {
                    if (segments.isNotEmpty() && segments.last() != ".." && segments.last() != "") {
                        segments.removeAt(segments.size - 1)
                    } else if (segments.isEmpty() || segments.last() == "..") {
                        segments.add("..")
                    }
                }
                else -> segments.add(segment)
            }
        }

        var result = segments.joinToString("/")
        if (result.isEmpty()) result = "."

        if (path.startsWith("/") && !result.startsWith("/")) {
            result = "/$result"
        }

        return result
    }

    private fun isUtf8Valid(input: String): Boolean {
        return try {
            val bytes = input.toByteArray(Charsets.UTF_8)
            val redecoded = String(bytes, Charsets.UTF_8)
            redecoded == input
        } catch (e: Exception) {
            false
        }
    }
}
