package com.amitia.amitia_app.runtime.proot

class ProotLaunchRequest private constructor(val rootfsPath: String, val workingDirectory: String, val commandSource: List<String>, val bindMountsSource: List<ProotBindMount>, val environmentSource: ProotEnvironment, val fakeRoot: Boolean, val killOnExit: Boolean) {
    val command: List<String> = commandSource.toList()
    val bindMounts: List<ProotBindMount> = bindMountsSource.toList()
    companion object {
        fun create(rootfsPath: String, workingDirectory: String, command: List<String>, bindMountsSource: List<ProotBindMount>, environmentSource: ProotEnvironment, fakeRoot: Boolean = true, killOnExit: Boolean = true): ProotLaunchRequest {
            require(rootfsPath.startsWith("/") && workingDirectory.startsWith("/") && command.isNotEmpty()) { "invalid request" }
            val guests = mutableSetOf<String>()
            for (m in bindMountsSource) require(guests.add(m.guest)) { "duplicate guest" }
            return ProotLaunchRequest(rootfsPath, workingDirectory, command.toList(), bindMountsSource.toList(), environmentSource, fakeRoot, killOnExit)
        }
    }
}