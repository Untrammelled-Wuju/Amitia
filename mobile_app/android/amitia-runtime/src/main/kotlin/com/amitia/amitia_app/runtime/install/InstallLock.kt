package com.amitia.amitia_app.runtime.install

import java.io.File
import java.io.RandomAccessFile
import java.nio.channels.FileChannel
import java.nio.channels.FileLock
import java.nio.channels.OverlappingFileLockException

internal interface InstallLock : AutoCloseable {
    fun isHeld(): Boolean

    companion object {
        fun acquire(layout: RuntimeHostLayout, timeoutMs: Long = 5000L): InstallLockResult {
            val lockFile = File(layout.locksRoot, RuntimeHostLayout.FILE_INSTALL_LOCK)
            lockFile.parentFile?.mkdirs()
            val raf = try {
                RandomAccessFile(lockFile, "rw")
            } catch (e: Exception) {
                return InstallLockResult.Failure(
                    RuntimeInstallErrorCode.IO_ERROR,
                    "failed to open lock file: ${e.message}"
                )
            }

            val channel: FileChannel = raf.channel
            val deadline = System.currentTimeMillis() + timeoutMs
            var lock: FileLock? = null

            while (true) {
                try {
                    lock = channel.tryLock()
                    if (lock != null) break
                } catch (_: OverlappingFileLockException) {
                } catch (e: Exception) {
                    return InstallLockResult.Failure(
                        RuntimeInstallErrorCode.IO_ERROR,
                        "failed to acquire lock: ${e.message}"
                    )
                }
                if (System.currentTimeMillis() >= deadline) {
                    return InstallLockResult.Failure(
                        RuntimeInstallErrorCode.INSTALL_ALREADY_IN_PROGRESS,
                        "another installation is in progress"
                    )
                }
                Thread.sleep(100)
            }

            return InstallLockResult.Success(FileBasedInstallLock(raf, lock))
        }

        fun acquire(paths: RuntimeInstallPaths, timeoutMs: Long = 5000L): InstallLockResult {
            val lockFile = File(paths.installLockFile())
            lockFile.parentFile?.mkdirs()
            val raf = try {
                RandomAccessFile(lockFile, "rw")
            } catch (e: Exception) {
                return InstallLockResult.Failure(
                    RuntimeInstallErrorCode.IO_ERROR,
                    "failed to open lock file: ${e.message}"
                )
            }

            val channel: FileChannel = raf.channel
            val deadline = System.currentTimeMillis() + timeoutMs
            var lock: FileLock? = null

            while (true) {
                try {
                    lock = channel.tryLock()
                    if (lock != null) break
                } catch (_: OverlappingFileLockException) {
                } catch (e: Exception) {
                    return InstallLockResult.Failure(
                        RuntimeInstallErrorCode.IO_ERROR,
                        "failed to acquire lock: ${e.message}"
                    )
                }
                if (System.currentTimeMillis() >= deadline) {
                    return InstallLockResult.Failure(
                        RuntimeInstallErrorCode.INSTALL_ALREADY_IN_PROGRESS,
                        "another installation is in progress"
                    )
                }
                Thread.sleep(100)
            }

            return InstallLockResult.Success(FileBasedInstallLock(raf, lock))
        }
    }
}

internal sealed interface InstallLockResult {
    data class Success(val lock: InstallLock) : InstallLockResult
    data class Failure(
        val code: RuntimeInstallErrorCode,
        val message: String,
    ) : InstallLockResult
}

internal class FileBasedInstallLock(
    private val raf: RandomAccessFile,
    private val lock: FileLock,
) : InstallLock {
    private val released = java.util.concurrent.atomic.AtomicBoolean(false)

    override fun isHeld(): Boolean = lock.isValid && !released.get()

    override fun close() {
        if (released.compareAndSet(false, true)) {
            try {
                lock.release()
            } catch (_: Exception) {
            }
            try {
                raf.close()
            } catch (_: Exception) {
            }
        }
    }
}
