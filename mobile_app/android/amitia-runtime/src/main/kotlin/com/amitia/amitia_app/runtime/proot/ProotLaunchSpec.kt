package com.amitia.amitia_app.runtime.proot

data class ProotLaunchSpec(
    val binaryPath: String,
    val rootfsPath: String,
    val workingDirectory: String,
    val command: List<String>,
    val bindMounts: List<ProotBindMount>,
    val environment: ProotEnvironment,
    val fakeRoot: Boolean = true,
    val killOnExit: Boolean = true,
) {
    companion object {
        const val DEFAULT_WORKDIR = GuestLayout.PROGRAM

        fun from(
            request: ProotLaunchRequest,
            binaryPath: String,
        ): ProotLaunchSpec {
            require(binaryPath.isNotBlank()) { "binaryPath must not be blank" }
            return ProotLaunchSpec(
                binaryPath = binaryPath,
                rootfsPath = request.rootfsPath,
                workingDirectory = request.workingDirectory,
                command = request.command,
                bindMounts = request.bindMounts,
                environment = request.environmentSource,
                fakeRoot = request.fakeRoot,
                killOnExit = request.killOnExit,
            )
        }
    }
}
