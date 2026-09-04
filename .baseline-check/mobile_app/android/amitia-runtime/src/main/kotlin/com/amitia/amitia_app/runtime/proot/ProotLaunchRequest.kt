package com.amitia.amitia_app.runtime.proot

class ProotLaunchRequest private constructor(val rootfsPath: String, val workingDirectory: String, val commandSource: List<String>, val bindMountsSource: List<ProotBindMount>, val environmentSource: ProotEnvironment, val fakeRoot: Boolean, val killOnExit: Boolean) {
    val command: List<String> = commandSource.toList()
    val bindMounts: List<ProotBindMount> = bindMountsSource.toList()
    companion object {
        private val SHELL_META_CHARS = setOf('|', '&', ';', '$', '`', '(', ')', '{', '}', '<', '>', '\\', '"', '\'')

        private fun isAbsoluteStyle(path: String): Boolean {
            if (path.startsWith("/")) return true
            if (path.length >= 2 && path[1] == ':') return true
            return false
        }

        fun create(rootfsPath: String, workingDirectory: String, command: List<String>, bindMountsSource: List<ProotBindMount>, environmentSource: ProotEnvironment, fakeRoot: Boolean = true, killOnExit: Boolean = true): ProotLaunchRequest {
            require(isAbsoluteStyle(rootfsPath) && isAbsoluteStyle(workingDirectory) && command.isNotEmpty()) { "invalid request" }
            require(!rootfsPath.contains("\u0000") && !workingDirectory.contains("\u0000")) { "nul byte in path" }
            for (arg in command) require(!arg.contains("\u0000")) { "nul byte in command" }
            val executable = command.first()
            require(executable.startsWith("/")) { "guest executable must be absolute" }
            require(!executable.contains("..")) { "guest executable must not contain traversal" }
            for (ch in SHELL_META_CHARS) require(!executable.contains(ch)) { "guest executable must not contain shell chars" }
            val guests = mutableSetOf<String>()
            for (m in bindMountsSource) require(guests.add(m.guest)) { "duplicate guest" }
            return ProotLaunchRequest(rootfsPath, workingDirectory, command.toList(), bindMountsSource.toList(), environmentSource, fakeRoot, killOnExit)
        }
    }
}