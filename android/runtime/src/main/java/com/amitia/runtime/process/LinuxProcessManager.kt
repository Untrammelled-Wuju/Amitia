package com.amitia.runtime.process

import kotlinx.coroutines.flow.Flow
import java.io.File

interface LinuxProcessManager {

    suspend fun start(
        name: String,
        command: List<String>,
        env: Map<String, String>,
        workDir: File,
        restartPolicy: RestartPolicy = RestartPolicy.NEVER,
        timeoutMs: Long? = null
    ): Result<Int>

    suspend fun stop(name: String, timeoutMs: Long = 5000L): Result<Unit>

    suspend fun forceStop(name: String): Result<Unit>

    fun status(name: String): ProcessStatus

    fun isAlive(name: String): Boolean

    fun crashCount(name: String): Int

    fun lastStartedAt(name: String): Long?

    fun lastExitReason(name: String): String?

    fun lastExitCode(name: String): Int?

    fun logs(name: String, tailLines: Int = 200): List<String>

    fun logFiles(name: String): List<File>

    suspend fun releaseAll()

    fun observeStatus(name: String): Flow<ProcessStatus>

    fun observeStdout(name: String): Flow<String>

    fun observeStderr(name: String): Flow<String>

    enum class ProcessStatus { RUNNING, STOPPED, CRASHED, UNKNOWN }
}
