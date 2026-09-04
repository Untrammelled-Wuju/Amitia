package com.amitia.amitia_app.runtime.proot

import com.amitia.amitia_app.runtime.install.RuntimeHostLayout
import java.io.File

enum class MountRole {
    PROGRAM,
    CONFIG,
    DATA,
    CACHE,
    LOGS,
    RUN,
    HOME,
}

data class MountSpec(
    val role: MountRole,
    val hostSource: String,
    val guestTarget: String,
    val writable: Boolean,
)

data class MountContract(
    val mounts: List<MountSpec>,
) {
    private val byRole: Map<MountRole, MountSpec> = mounts.associateBy { it.role }
    private val byGuest: Map<String, MountSpec> = mounts.associateBy { it.guestTarget }

    val programMount: MountSpec get() = byRole.getValue(MountRole.PROGRAM)
    val configMount: MountSpec get() = byRole.getValue(MountRole.CONFIG)
    val dataMount: MountSpec get() = byRole.getValue(MountRole.DATA)
    val cacheMount: MountSpec get() = byRole.getValue(MountRole.CACHE)
    val logsMount: MountSpec get() = byRole.getValue(MountRole.LOGS)
    val runMount: MountSpec get() = byRole.getValue(MountRole.RUN)
    val homeMount: MountSpec get() = byRole.getValue(MountRole.HOME)

    val guestTargets: List<String> get() = mounts.map { it.guestTarget }

    fun toProotBindMounts(): List<ProotBindMount> = mounts.map {
        ProotBindMount.create(it.hostSource, it.guestTarget, readOnly = !it.writable)
    }

    companion object {
        internal fun build(
            hostLayout: RuntimeHostLayout,
            activeProgramSource: File,
        ): MountContract {
            validateProgramSource(hostLayout, activeProgramSource)

            val mounts = listOf(
                MountSpec(MountRole.PROGRAM, activeProgramSource.absolutePath, GuestLayout.PROGRAM, false),
                MountSpec(MountRole.CONFIG, hostLayout.configRoot.absolutePath, GuestLayout.CONFIG, true),
                MountSpec(MountRole.DATA, hostLayout.dataRoot.absolutePath, GuestLayout.DATA, true),
                MountSpec(MountRole.CACHE, hostLayout.cacheRoot.absolutePath, GuestLayout.CACHE, true),
                MountSpec(MountRole.LOGS, hostLayout.logRoot.absolutePath, GuestLayout.LOGS, true),
                MountSpec(MountRole.RUN, hostLayout.runRoot.absolutePath, GuestLayout.RUN, true),
                MountSpec(MountRole.HOME, hostLayout.homeRoot.absolutePath, GuestLayout.HOME, true),
            )

            validateMounts(mounts)
            return MountContract(mounts)
        }

        private fun validateProgramSource(hostLayout: RuntimeHostLayout, source: File) {
            val sourcePath = source.absolutePath
            require(sourcePath.isNotBlank()) { "active program source must not be blank" }
            require(sourcePath != hostLayout.versionsRoot.absolutePath) {
                "active program source must not be versionsRoot directly; must be versions/<version>"
            }
            require(sourcePath.startsWith(hostLayout.versionsRoot.absolutePath + "/")) {
                "active program source must be within versionsRoot"
            }
            require(!sourcePath.contains("..")) { "active program source must not contain traversal" }
            require(source.isDirectory) { "active program source must be a directory: $sourcePath" }
            require(source.exists()) { "active program source must exist: $sourcePath" }
        }

        private fun validateMounts(mounts: List<MountSpec>) {
            val targets = mounts.map { it.guestTarget }
            require(targets.size == targets.toSet().size) {
                "duplicate guest targets detected"
            }
            for (mount in mounts) {
                require(mount.guestTarget.startsWith("/")) { "guest target must be absolute: ${mount.guestTarget}" }
                require(mount.guestTarget != "/") { "guest target must not be root" }
                require(!mount.guestTarget.contains("..")) { "guest target must not contain traversal" }
                require(mount.hostSource.isNotBlank()) { "host source must not be blank" }
            }
        }
    }
}
