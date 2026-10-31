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
                } catch (originalError33: OverlappingFileLockException) {
                  originalError33.printStackTrace()

                } catch (e: Exception) {
                    try { raf.close() } catch (originalError35: Exception) {
                      originalError35.printStackTrace()
                    }
                    return InstallLockResult.Failure(
                        RuntimeInstallErrorCode.IO_ERROR,
                        "failed to acquire lock: ${e.message}"
                    )
                }
                if (System.currentTimeMillis() >= deadline) {
                    try { raf.close() } catch (originalError42: Exception) {
                      originalError42.printStackTrace()
                    }
                    return InstallLockResult.Failure(
                        RuntimeInstallErrorCode.INSTALL_ALREADY_IN_PROGRESS,
                        "another installation is in progress"
                    )
                }
                Thread.sleep(100)
            }

            val acquiredLock = lock ?: run {
                try { raf.close() } catch (originalError52: Exception) {
                  originalError52.printStackTrace()
                }
                return InstallLockResult.Failure(
                    RuntimeInstallErrorCode.IO_ERROR,
                    "lock acquisition completed without a lock",
                )
            }
            return InstallLockResult.Success(FileBasedInstallLock(raf, acquiredLock))
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
                } catch (originalError81: OverlappingFileLockException) {
                  originalError81.printStackTrace()

                } catch (e: Exception) {
                    try { raf.close() } catch (originalError83: Exception) {
                      originalError83.printStackTrace()
                    }
                    return InstallLockResult.Failure(
                        RuntimeInstallErrorCode.IO_ERROR,
                        "failed to acquire lock: ${e.message}"
                    )
                }
                if (System.currentTimeMillis() >= deadline) {
                    try { raf.close() } catch (originalError90: Exception) {
                      originalError90.printStackTrace()
                    }
                    return InstallLockResult.Failure(
                        RuntimeInstallErrorCode.INSTALL_ALREADY_IN_PROGRESS,
                        "another installation is in progress"
                    )
                }
                Thread.sleep(100)
            }

            val acquiredLock = lock ?: run {
                try { raf.close() } catch (originalError100: Exception) {
                  originalError100.printStackTrace()
                }
                return InstallLockResult.Failure(
                    RuntimeInstallErrorCode.IO_ERROR,
                    "lock acquisition completed without a lock",
                )
            }
            return InstallLockResult.Success(FileBasedInstallLock(raf, acquiredLock))
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
            } catch (originalError131: Exception) {
              originalError131.printStackTrace()

            }
            try {
                raf.close()
            } catch (originalError135: Exception) {
              originalError135.printStackTrace()

            }
        }
    }
}
