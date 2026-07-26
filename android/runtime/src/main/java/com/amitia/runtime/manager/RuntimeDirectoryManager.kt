package com.amitia.runtime.manager

import android.content.Context
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.File
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class RuntimeDirectoryManager @Inject constructor(
    @ApplicationContext private val context: Context
) {

    private val filesDir: File = context.filesDir

    fun runtimeRoot(): File = File(filesDir, "runtime")

    fun rootfsDir(): File = File(runtimeRoot(), "rootfs")

    fun binDir(): File = File(runtimeRoot(), "bin")

    fun logsDir(): File = File(runtimeRoot(), "logs")

    fun tmpDir(): File = File(runtimeRoot(), "tmp")

    fun versionsDir(): File = File(runtimeRoot(), "versions")

    fun configDir(): File = File(runtimeRoot(), "config")

    fun amitiaDataRoot(): File = File(filesDir, "amitia-data")

    fun sqliteDir(): File = File(amitiaDataRoot(), "sqlite")

    fun qdrantDir(): File = File(amitiaDataRoot(), "qdrant")

    fun surrealdbDir(): File = File(amitiaDataRoot(), "surrealdb")

    fun uploadsDir(): File = File(amitiaDataRoot(), "uploads")

    fun modelsDir(): File = File(amitiaDataRoot(), "models")

    fun extensionsDir(): File = File(amitiaDataRoot(), "extensions")

    fun backupsDir(): File = File(amitiaDataRoot(), "backups")

    fun runtimeDirectories(): List<File> = listOf(
        runtimeRoot(),
        rootfsDir(),
        binDir(),
        logsDir(),
        tmpDir(),
        versionsDir(),
        configDir()
    )

    fun userDataDirectories(): List<File> = listOf(
        amitiaDataRoot(),
        sqliteDir(),
        qdrantDir(),
        surrealdbDir(),
        uploadsDir(),
        modelsDir(),
        extensionsDir(),
        backupsDir()
    )

    fun allDirectories(): List<File> = runtimeDirectories() + userDataDirectories()

    suspend fun ensureDirectories(): Result<Unit> = withContext(Dispatchers.IO) {
        try {
            for (dir in allDirectories()) {
                if (!dir.exists()) {
                    val created = dir.mkdirs()
                    if (!created && !dir.exists()) {
                        return@withContext Result.failure(
                            IllegalStateException("无法创建目录: ${dir.absolutePath}")
                        )
                    }
                }
                if (!dir.canWrite()) {
                    return@withContext Result.failure(
                        IllegalStateException("目录不可写: ${dir.absolutePath}")
                    )
                }
                applyDirectoryPermissions(dir)
            }
            Result.success(Unit)
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    fun validateIsolation(): Boolean {
        val rootfsCanonical = try {
            rootfsDir().canonicalPath
        } catch (_: Exception) {
            rootfsDir().absolutePath
        }
        val dataCanonical = try {
            amitiaDataRoot().canonicalPath
        } catch (_: Exception) {
            amitiaDataRoot().absolutePath
        }
        return !dataCanonical.startsWith(rootfsCanonical)
    }

    fun validatePortIsolation(): Boolean = true

    private fun applyDirectoryPermissions(dir: File) {
        try {
            dir.setReadable(true, true)
            dir.setWritable(true, true)
            dir.setExecutable(true, true)
        } catch (_: Exception) {
        }
    }
}

@Singleton
class RuntimeDirectories @Inject constructor(
    @ApplicationContext private val context: Context
) {

    private val manager: RuntimeDirectoryManager by lazy { RuntimeDirectoryManager(context) }

    fun runtimeRoot(): File = manager.runtimeRoot()

    fun rootfsDir(): File = manager.rootfsDir()

    fun binDir(): File = manager.binDir()

    fun logsDir(): File = manager.logsDir()

    fun tmpDir(): File = manager.tmpDir()

    fun versionsDir(): File = manager.versionsDir()

    fun configDir(): File = manager.configDir()

    fun amitiaDataRoot(): File = manager.amitiaDataRoot()

    fun sqliteDir(): File = manager.sqliteDir()

    fun qdrantDir(): File = manager.qdrantDir()

    fun surrealdbDir(): File = manager.surrealdbDir()

    fun uploadsDir(): File = manager.uploadsDir()

    fun modelsDir(): File = manager.modelsDir()

    fun extensionsDir(): File = manager.extensionsDir()

    fun backupsDir(): File = manager.backupsDir()

    fun allDirectories(): List<File> = manager.allDirectories()

    suspend fun ensureAllCreated(): Result<Unit> = manager.ensureDirectories()

    fun validateIsolation(): Boolean = manager.validateIsolation()

    fun validatePortIsolation(): Boolean = manager.validatePortIsolation()
}
