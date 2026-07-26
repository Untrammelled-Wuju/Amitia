package com.amitia.runtime.process

import com.amitia.runtime.api.RuntimeEvent
import com.amitia.runtime.linux.LinuxRootfsManager
import com.amitia.runtime.linux.ProotBinaryManager
import com.amitia.runtime.manager.RuntimeStateMachine
import java.io.File
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class ProotCommandWrapperImpl @Inject constructor(
    private val prootBinaryManager: ProotBinaryManager,
    private val rootfsManager: LinuxRootfsManager,
    private val stateMachine: RuntimeStateMachine
) : ProotCommandWrapper {

    override fun isProotAvailable(): Boolean = prootBinaryManager.isAvailable()

    override fun fallbackReason(): String? {
        return prootBinaryManager.unavailableReason()
    }

    override fun wrap(
        command: List<String>,
        env: Map<String, String>,
        workDir: File
    ): List<String> {
        if (command.isEmpty()) return command
        if (!prootBinaryManager.isAvailable()) {
            val reason = prootBinaryManager.unavailableReason() ?: "PRoot 二进制不可用"
            stateMachine.emitError(
                error = "PRoot 包装失败: $reason",
                retryable = false,
                requiresUserAction = true
            )
            return command
        }
        val prootPath = prootBinaryManager.binaryPath()
            ?: return command
        val rootfs = rootfsManager.minimalRootfsDir()

        val wrapped = mutableListOf<String>()
        wrapped.add(prootPath.absolutePath)
        wrapped.add("--rootfs=${rootfs.absolutePath}")
        wrapped.add("--root-id")
        wrapped.add("--cwd=${workDir.absolutePath}")
        wrapped.add("--bind=/dev")
        wrapped.add("--bind=/proc")
        wrapped.add("--bind=/sys")
        wrapped.add("--bind=${rootfs.absolutePath}:/")
        wrapped.add("--bind=${workDir.absolutePath}:${workDir.absolutePath}")

        val dataBindTarget = rootfs.parentFile?.parentFile?.parentFile
        if (dataBindTarget != null && dataBindTarget.exists()) {
            wrapped.add("--bind=${dataBindTarget.absolutePath}:${dataBindTarget.absolutePath}")
        }

        if (env.isNotEmpty()) {
            wrapped.add("--env")
            env.forEach { (k, v) ->
                wrapped.add("$k=$v")
            }
        }

        wrapped.add("--")
        wrapped.addAll(command)

        stateMachine.emitLog(
            RuntimeEvent.LogEmitted.Level.INFO,
            TAG,
            "PRoot 包装命令 rootfs=${rootfs.absolutePath} workDir=${workDir.absolutePath} cmd=${command.joinToString(" ")}"
        )

        return wrapped
    }

    companion object {
        private const val TAG = "ProotWrapper"
    }
}
