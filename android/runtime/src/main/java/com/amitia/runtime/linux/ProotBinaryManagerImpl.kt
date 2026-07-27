package com.amitia.runtime.linux

import android.content.Context
import android.system.Os
import com.amitia.runtime.api.RuntimeEvent
import com.amitia.runtime.manager.RuntimeStateMachine
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.callbackFlow
import kotlinx.coroutines.flow.flowOn
import kotlinx.coroutines.withContext
import java.io.File
import java.io.FileOutputStream
import java.security.MessageDigest
import java.util.concurrent.TimeUnit
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class ProotBinaryManagerImpl @Inject constructor(
    @ApplicationContext private val context: Context,
    private val stateMachine: RuntimeStateMachine,
    private val integrityChecker: RootfsIntegrityChecker
) : ProotBinaryManager {

    private val filesDir: File = context.filesDir

    private val binDir: File = File(filesDir, "runtime/bin")

    private val targetFile: File = File(binDir, PROOT_BINARY_NAME)

    private val versionFile: File = File(binDir, PROOT_VERSION_FILE_NAME)

    private val nativeLibDir: File = File(context.applicationInfo.nativeLibraryDir)

    private val execDir: File = File("/data/local/tmp/amitia")

    private val execFile: File = File(execDir, PROOT_BINARY_NAME)

    private val nativeExecFile: File = File(nativeLibDir, "libproot_exec.so")

    override fun isAvailable(): Boolean {
        if (nativeExecFile.exists() && nativeExecFile.canExecute()) return true
        if (!targetFile.exists()) return false
        val expected = readExpectedSha256() ?: return true
        val actual = integrityChecker.sha256(targetFile)
        return actual.equals(expected, ignoreCase = true)
    }

    override fun unavailableReason(): String? {
        if (nativeExecFile.exists() && nativeExecFile.canExecute()) return null
        if (!assetExists(PROOT_ASSET_NAME)) {
            return "PRoot 二进制未预置,请下载 proot-rs ARM64 静态版本放入 assets/ (assets/$PROOT_ASSET_NAME 缺失)。获取方式: https://github.com/proot-me/proot-rs/releases 下载 proot-rs-aarch64-linux-static,或从 Termux packages 提取 proot-aarch64 静态二进制。"
        }
        if (!targetFile.exists()) {
            return "PRoot 二进制尚未安装,请调用 install() 后重试"
        }
        val expected = readExpectedSha256()
        if (expected != null) {
            val actual = integrityChecker.sha256(targetFile)
            if (!actual.equals(expected, ignoreCase = true)) {
                return "PRoot 二进制 SHA-256 校验失败 expected=$expected actual=$actual"
            }
        }
        val execPath = resolveExecPath()
        if (execPath == null) {
            return "PRoot 二进制无法复制到可执行位置 (noexec 文件系统限制)"
        }
        return null
    }

    override fun version(): String? {
        if (!targetFile.exists()) return null
        return if (versionFile.exists()) versionFile.readText().trim().ifEmpty { null }
        else PROOT_DEFAULT_VERSION
    }

    override fun binaryPath(): File? {
        if (!isAvailable()) return null
        val path = resolveExecPath()
        if (path != null) return path
        if (targetFile.exists()) return targetFile
        return null
    }

    override fun verify(): Result<Unit> {
        val reason = unavailableReason()
        if (reason != null) {
            return Result.failure(IllegalStateException(reason))
        }
        return Result.success(Unit)
    }

    override fun install(): Flow<ProotInstallProgress> = callbackFlow {
        trySend(
            ProotInstallProgress(
                phase = ProotInstallPhase.STARTED,
                percent = 0f,
                message = "开始安装 PRoot 二进制"
            )
        )
        try {
            if (nativeExecFile.exists() && nativeExecFile.canExecute()) {
                stateMachine.emitLog(
                    RuntimeEvent.LogEmitted.Level.INFO,
                    TAG,
                    "PRoot jniLibs 预置版本已可用: ${nativeExecFile.absolutePath}"
                )
                trySend(
                    ProotInstallProgress(
                        phase = ProotInstallPhase.COMPLETED,
                        percent = 1f,
                        message = "PRoot 已通过 jniLibs 就绪: $PROOT_DEFAULT_VERSION"
                    )
                )
                close()
                return@callbackFlow
            }

            if (!assetExists(PROOT_ASSET_NAME)) {
                val msg = "PRoot 二进制未预置,请下载 proot-rs ARM64 静态版本放入 assets/ (assets/$PROOT_ASSET_NAME 缺失)"
                stateMachine.emitError(
                    error = msg,
                    retryable = false,
                    requiresUserAction = true
                )
                trySend(
                    ProotInstallProgress(
                        phase = ProotInstallPhase.FAILED,
                        percent = 0f,
                        message = msg,
                        error = msg
                    )
                )
                close(IllegalStateException(msg))
                return@callbackFlow
            }

            binDir.mkdirs()
            val expectedSha = readExpectedSha256()

            val assetTotal = try {
                context.assets.openFd(PROOT_ASSET_NAME).use { it.length }
            } catch (_: Exception) {
                -1L
            }
            val buffer = ByteArray(BUFFER_SIZE)
            var copied = 0L
            context.assets.open(PROOT_ASSET_NAME).use { input ->
                FileOutputStream(targetFile).use { out ->
                    while (true) {
                        val read = input.read(buffer)
                        if (read == -1) break
                        out.write(buffer, 0, read)
                        copied += read
                        val pct = if (assetTotal > 0) copied.toFloat() / assetTotal.toFloat() else 0f
                        trySend(
                            ProotInstallProgress(
                                phase = ProotInstallPhase.COPYING,
                                percent = (pct * 0.7f).coerceIn(0f, 0.7f),
                                message = "复制 PRoot 二进制 ($copied bytes)"
                            )
                        )
                    }
                }
            }

            trySend(
                ProotInstallProgress(
                    phase = ProotInstallPhase.VERIFYING,
                    percent = 0.85f,
                    message = "校验 PRoot 二进制 SHA-256"
                )
            )

            if (expectedSha != null) {
                val actualSha = integrityChecker.sha256(targetFile)
                if (!actualSha.equals(expectedSha, ignoreCase = true)) {
                    val msg = "PRoot 二进制 SHA-256 不匹配 expected=$expectedSha actual=$actualSha"
                    stateMachine.emitError(
                        error = msg,
                        retryable = false,
                        requiresUserAction = true
                    )
                    trySend(
                        ProotInstallProgress(
                            phase = ProotInstallPhase.FAILED,
                            percent = 0.85f,
                            message = msg,
                            error = msg
                        )
                    )
                    close(IllegalStateException(msg))
                    return@callbackFlow
                }
            }
            versionFile.writeText(PROOT_DEFAULT_VERSION)

            stateMachine.emitLog(
                RuntimeEvent.LogEmitted.Level.INFO,
                TAG,
                "PRoot 二进制安装完成 path=${targetFile.absolutePath} size=${targetFile.length()}"
            )

            val execPath = deployExecCopy()
            if (execPath != null) {
                stateMachine.emitLog(
                    RuntimeEvent.LogEmitted.Level.INFO,
                    TAG,
                    "PRoot 可执行副本已部署到 /data/local/tmp: ${execPath.absolutePath}"
                )
            } else {
                stateMachine.emitLog(
                    RuntimeEvent.LogEmitted.Level.WARN,
                    TAG,
                    "PRoot 无法部署到 /data/local/tmp,若设备支持 native lib 执行请确保 jniLibs 已内嵌"
                )
            }

            trySend(
                ProotInstallProgress(
                    phase = ProotInstallPhase.COMPLETED,
                    percent = 1f,
                    message = "PRoot 安装完成: $PROOT_DEFAULT_VERSION"
                )
            )
            close()
        } catch (e: Exception) {
            stateMachine.emitError(
                error = "PRoot 二进制安装失败: ${e.message}",
                retryable = true,
                requiresUserAction = false,
                cause = e
            )
            trySend(
                ProotInstallProgress(
                    phase = ProotInstallPhase.FAILED,
                    percent = 0f,
                    message = "PRoot 安装失败: ${e.message}",
                    error = e.message
                )
            )
            close(e)
        }
        awaitClose { }
    }.flowOn(Dispatchers.IO)

    private fun deployExecCopy(): File? {
        return tryCopyToExecLocation(execFile)
    }

    private fun tryCopyToExecLocation(dest: File): File? {
        return try {
            if (!targetFile.exists()) return null
            dest.parentFile?.mkdirs()
            targetFile.copyTo(dest, overwrite = true)
            ensureExecutable(dest)
            if (!dest.canExecute()) {
                try { Os.chmod(dest.absolutePath, 493) } catch (_: Exception) {}
            }
            if (!dest.canExecute()) return null
            val testOk = try {
                val pb = ProcessBuilder(dest.absolutePath, "--version")
                pb.redirectErrorStream(true)
                val p = pb.start()
                val exited = p.waitFor(5, TimeUnit.SECONDS)
                if (!exited) { p.destroyForcibly(); false } else { p.exitValue() == 0 }
            } catch (_: Exception) {
                false
            }
            if (!testOk) {
                dest.delete()
                return null
            }
            dest
        } catch (e: Exception) {
            stateMachine.emitLog(
                RuntimeEvent.LogEmitted.Level.WARN,
                TAG,
                "复制 PRoot 到 ${dest.absolutePath} 失败: ${e.message}"
            )
            null
        }
    }

    private fun resolveExecPath(): File? {
        if (nativeExecFile.exists() && nativeExecFile.canExecute()) {
            return nativeExecFile
        }
        if (execFile.exists() && execFile.canExecute()) {
            if (execFile.lastModified() >= targetFile.lastModified()) {
                return execFile
            }
        }
        val deployed = deployExecCopy()
        if (deployed != null) return deployed
        if (execFile.exists() && execFile.canExecute()) return execFile
        if (nativeExecFile.exists() && nativeExecFile.canExecute()) return nativeExecFile
        return null
    }

    private fun assetExists(name: String): Boolean {
        return try {
            context.assets.open(name).use { }
            true
        } catch (_: Exception) {
            false
        }
    }

    private fun readExpectedSha256(): String? {
        return try {
            context.assets.open(PROOT_SHA256_ASSET_NAME).use { stream ->
                stream.bufferedReader().readText()
                    .trim()
                    .split(Regex("\\s+"))
                    .firstOrNull { it.isNotEmpty() }
                    ?.ifEmpty { null }
            }
        } catch (_: Exception) {
            null
        }
    }

    private fun ensureExecutable(file: File) {
        try {
            Os.chmod(file.absolutePath, 493)
        } catch (_: Exception) {
            file.setExecutable(true, false)
        }
    }

    companion object {
        private const val TAG = "ProotBinaryManager"
        const val PROOT_BINARY_NAME = "proot"
        const val PROOT_ASSET_NAME = "proot_linux_aarch64"
        const val PROOT_SHA256_ASSET_NAME = "proot_linux_aarch64.sha256"
        const val PROOT_VERSION_FILE_NAME = ".amitia-proot-version"
        const val PROOT_DEFAULT_VERSION = "proot-rs-aarch64-static-0.1.0"
        private const val BUFFER_SIZE = 64 * 1024
    }
}
