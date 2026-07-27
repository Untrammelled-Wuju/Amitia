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
import kotlinx.serialization.json.Json
import java.io.File
import java.io.FileOutputStream
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class LinuxRootfsManagerImpl @Inject constructor(
    @ApplicationContext private val context: Context,
    private val stateMachine: RuntimeStateMachine,
    private val integrityChecker: RootfsIntegrityChecker
) : LinuxRootfsManager {

    private val filesDir: File = context.filesDir

    private val json = Json {
        prettyPrint = true
        ignoreUnknownKeys = true
        encodeDefaults = true
    }

    override fun rootfsDir(): File = File(filesDir, "runtime/rootfs")

    override fun binDir(): File = File(filesDir, "runtime/bin")

    override fun versionsDir(): File = File(filesDir, "runtime/versions")

    override fun versionFile(): File = File(rootfsDir(), VERSION_FILE_NAME)

    private fun manifestFile(): File = File(rootfsDir(), MANIFEST_FILE_NAME)

    private fun stagingDir(tag: String): File =
        File(filesDir, "runtime/.staging-$tag-${System.currentTimeMillis()}")

    override suspend fun isInstalled(): Boolean = withContext(Dispatchers.IO) {
        val rootfs = rootfsDir()
        val bin = binDir()
        val vf = versionFile()
        val manifest = manifestFile()
        if (!rootfs.exists() || !bin.exists() || !vf.exists() || !manifest.exists()) {
            return@withContext false
        }
        val components = readManifest()?.components ?: return@withContext false
        components.all { component ->
            val target = if (component.type == TYPE_CONFIG) {
                File(rootfs, component.file)
            } else {
                File(bin, component.file)
            }
            target.exists() && (component.type == TYPE_CONFIG || target.canExecute())
        }
    }

    override suspend fun getCurrentVersion(): String? = withContext(Dispatchers.IO) {
        val vf = versionFile()
        if (!vf.exists()) null
        else vf.readText().trim().ifEmpty { null }
    }

    override suspend fun getManifestVersion(): String? = withContext(Dispatchers.IO) {
        readManifest()?.version
    }

    override suspend fun install(): Flow<RootfsInstallProgress> = callbackFlow {
        val manifest = try {
            readManifest()
        } catch (_: Exception) {
            null
        }
        val totalBytes = manifest?.totalFilesSize() ?: 0L
        var copied: Long = 0L
        try {
            trySend(
                RootfsInstallProgress(
                    phase = RootfsInstallPhase.STARTED,
                    currentFile = "",
                    bytesCopied = 0L,
                    totalBytes = totalBytes,
                    percent = 0f,
                    message = "开始安装 RootFS"
                )
            )

            cleanupResidual()
            rootfsDir().mkdirs()
            binDir().mkdirs()
            versionsDir().mkdirs()

            val resolvedManifest = manifest
                ?: throw IllegalStateException("assets/$MANIFEST_ASSET_NAME 缺失或解析失败")

            val components = resolvedManifest.components
            val totalCount = components.size
            for ((index, component) in components.withIndex()) {
                val targetDir = if (component.type == TYPE_CONFIG) rootfsDir() else binDir()
                targetDir.mkdirs()
                val targetFile = File(targetDir, component.file)

                val beforeBytes = copied
                copyAssetWithProgress(component.file, targetFile) { delta, total ->
                    copied = beforeBytes + delta
                    val pct = if (totalBytes > 0) {
                        copied.toFloat() / totalBytes.toFloat()
                    } else 0f
                    trySend(
                        RootfsInstallProgress(
                            phase = RootfsInstallPhase.COPYING,
                            currentFile = component.file,
                            bytesCopied = copied,
                            totalBytes = totalBytes,
                            percent = pct.coerceIn(0f, 1f),
                            message = "解压 ${component.name} (${index + 1}/$totalCount)"
                        )
                    )
                }

                if (component.type != TYPE_CONFIG) {
                    ensureExecutable(targetFile)
                }

                trySend(
                    RootfsInstallProgress(
                        phase = RootfsInstallPhase.VERIFYING,
                        currentFile = component.file,
                        bytesCopied = copied,
                        totalBytes = totalBytes,
                        percent = (copied.toFloat() / totalBytes.coerceAtLeast(1)).coerceIn(0f, 1f),
                        message = "校验 ${component.name} SHA-256"
                    )
                )
                val actualSha = integrityChecker.sha256(targetFile)
                if (!actualSha.equals(component.sha256, ignoreCase = true)) {
                    throw IllegalStateException(
                        "组件 ${component.name} SHA-256 不匹配 expected=${component.sha256} actual=$actualSha"
                    )
                }
            }

            trySend(
                RootfsInstallProgress(
                    phase = RootfsInstallPhase.FINALIZING,
                    currentFile = "",
                    bytesCopied = copied,
                    totalBytes = totalBytes,
                    percent = 0.98f,
                    message = "写入版本与清单"
                )
            )

            versionFile().writeText(resolvedManifest.version)
            manifestFile().writeText(json.encodeToString(RootfsManifest.serializer(), resolvedManifest))
            File(versionsDir(), "${resolvedManifest.version}.txt").writeText(
                buildString {
                    append("installedAt=").append(System.currentTimeMillis()).append('\n')
                    append("version=").append(resolvedManifest.version).append('\n')
                    append("source=assets").append('\n')
                }
            )

            stateMachine.emitLog(
                RuntimeEvent.LogEmitted.Level.INFO,
                TAG,
                "RootFS 安装完成 version=${resolvedManifest.version} components=${components.size} bytes=$copied"
            )

            trySend(
                RootfsInstallProgress(
                    phase = RootfsInstallPhase.COMPLETED,
                    currentFile = "",
                    bytesCopied = copied,
                    totalBytes = totalBytes,
                    percent = 1f,
                    message = "RootFS 安装完成: ${resolvedManifest.version}"
                )
            )
            close()
        } catch (e: Exception) {
            stateMachine.emitError(
                error = "RootFS 安装失败: ${e.message}",
                retryable = true,
                requiresUserAction = false,
                cause = e
            )
            trySend(
                RootfsInstallProgress(
                    phase = RootfsInstallPhase.FAILED,
                    currentFile = "",
                    bytesCopied = copied,
                    totalBytes = totalBytes,
                    percent = if (totalBytes > 0) copied.toFloat() / totalBytes else 0f,
                    message = "RootFS 安装失败: ${e.message}",
                    error = e.message
                )
            )
            close(e)
        }
        awaitClose { }
    }.flowOn(Dispatchers.IO)

    override suspend fun verify(): Result<RootfsIntegrity> = withContext(Dispatchers.IO) {
        try {
            val manifest = readManifest()
            if (manifest == null) {
                return@withContext Result.success(
                    RootfsIntegrity(
                        valid = false,
                        missingFiles = listOf(MANIFEST_ASSET_NAME),
                        corruptedFiles = emptyList()
                    )
                )
            }
            val missing = mutableListOf<String>()
            val corrupted = mutableListOf<String>()
            for (component in manifest.components) {
                val target = if (component.type == TYPE_CONFIG) {
                    File(rootfsDir(), component.file)
                } else {
                    File(binDir(), component.file)
                }
                if (!target.exists()) {
                    missing.add(component.file)
                    continue
                }
                val actual = integrityChecker.sha256(target)
                if (!actual.equals(component.sha256, ignoreCase = true)) {
                    corrupted.add(component.file)
                }
            }
            val integrity = RootfsIntegrity(
                valid = missing.isEmpty() && corrupted.isEmpty(),
                missingFiles = missing,
                corruptedFiles = corrupted
            )
            stateMachine.emitLog(
                RuntimeEvent.LogEmitted.Level.INFO,
                TAG,
                "RootFS 校验 valid=${integrity.valid} missing=${integrity.missingFiles.size} corrupted=${integrity.corruptedFiles.size}"
            )
            Result.success(integrity)
        } catch (e: Exception) {
            stateMachine.emitError(
                error = "RootFS 校验失败: ${e.message}",
                retryable = true,
                requiresUserAction = false,
                cause = e
            )
            Result.failure(e)
        }
    }

    override suspend fun upgrade(): Flow<RootfsInstallProgress> = callbackFlow {
        try {
            val manifest = readManifest()
                ?: throw IllegalStateException("升级失败: 清单缺失")
            val installedVersion = getCurrentVersion()
            if (installedVersion != null && installedVersion == manifest.version) {
                trySend(
                    RootfsInstallProgress(
                        phase = RootfsInstallPhase.COMPLETED,
                        currentFile = "",
                        bytesCopied = 0L,
                        totalBytes = 0L,
                        percent = 1f,
                        message = "版本已是最新: $installedVersion"
                    )
                )
                close()
                return@callbackFlow
            }

            val staging = stagingDir("rootfs-upgrade")
            staging.mkdirs()
            val stagingBin = File(staging, "bin").apply { mkdirs() }

            val totalBytes = manifest.totalFilesSize()
            var copied: Long = 0L
            trySend(
                RootfsInstallProgress(
                    phase = RootfsInstallPhase.STARTED,
                    currentFile = "",
                    bytesCopied = 0L,
                    totalBytes = totalBytes,
                    percent = 0f,
                    message = "开始升级 RootFS: $installedVersion -> ${manifest.version}"
                )
            )

            for ((index, component) in manifest.components.withIndex()) {
                val targetDir = if (component.type == TYPE_CONFIG) staging else stagingBin
                val targetFile = File(targetDir, component.file)
                val before = copied
                copyAssetWithProgress(component.file, targetFile) { delta, _ ->
                    copied = before + delta
                    val pct = if (totalBytes > 0) copied.toFloat() / totalBytes else 0f
                    trySend(
                        RootfsInstallProgress(
                            phase = RootfsInstallPhase.COPYING,
                            currentFile = component.file,
                            bytesCopied = copied,
                            totalBytes = totalBytes,
                            percent = pct.coerceIn(0f, 1f),
                            message = "升级解压 ${component.name} (${index + 1}/${manifest.components.size})"
                        )
                    )
                }
                if (component.type != TYPE_CONFIG) {
                    targetFile.setExecutable(true, false)
                }
                val actual = integrityChecker.sha256(targetFile)
                if (!actual.equals(component.sha256, ignoreCase = true)) {
                    throw IllegalStateException("升级包校验失败: ${component.name}")
                }
            }

            trySend(
                RootfsInstallProgress(
                    phase = RootfsInstallPhase.FINALIZING,
                    currentFile = "",
                    bytesCopied = copied,
                    totalBytes = totalBytes,
                    percent = 0.95f,
                    message = "替换 RootFS 目录(保留用户数据)"
                )
            )

            val currentBin = binDir()
            val backupBin = stagingDir("bin-bak")
            val replaced = if (currentBin.exists()) {
                currentBin.renameTo(backupBin) && stagingBin.renameTo(currentBin)
            } else {
                stagingBin.renameTo(currentBin)
            }
            if (!replaced) {
                if (backupBin.exists() && !currentBin.exists()) {
                    backupBin.renameTo(currentBin)
                }
                staging.deleteRecursively()
                throw IllegalStateException("RootFS bin 原子替换失败")
            }
            backupBin.deleteRecursively()

            rootfsDir().mkdirs()
            versionFile().writeText(manifest.version)
            manifestFile().writeText(json.encodeToString(RootfsManifest.serializer(), manifest))
            File(versionsDir(), "${manifest.version}.txt").writeText(
                buildString {
                    append("upgradedAt=").append(System.currentTimeMillis()).append('\n')
                    append("from=").append(installedVersion ?: "unknown").append('\n')
                    append("to=").append(manifest.version).append('\n')
                }
            )
            staging.deleteRecursively()

            stateMachine.emitLog(
                RuntimeEvent.LogEmitted.Level.INFO,
                TAG,
                "RootFS 升级完成 from=$installedVersion to=${manifest.version}"
            )

            trySend(
                RootfsInstallProgress(
                    phase = RootfsInstallPhase.COMPLETED,
                    currentFile = "",
                    bytesCopied = copied,
                    totalBytes = totalBytes,
                    percent = 1f,
                    message = "RootFS 升级完成: ${manifest.version}"
                )
            )
            close()
        } catch (e: Exception) {
            stateMachine.emitError(
                error = "RootFS 升级失败: ${e.message}",
                retryable = true,
                requiresUserAction = false,
                cause = e
            )
            trySend(
                RootfsInstallProgress(
                    phase = RootfsInstallPhase.FAILED,
                    currentFile = "",
                    bytesCopied = 0L,
                    totalBytes = 0L,
                    percent = 0f,
                    message = "RootFS 升级失败: ${e.message}",
                    error = e.message
                )
            )
            close(e)
        }
        awaitClose { }
    }.flowOn(Dispatchers.IO)

    override suspend fun cleanup(requireConfirmation: Boolean): Result<Unit> = withContext(Dispatchers.IO) {
        try {
            val dir = rootfsDir()
            val bin = binDir()
            val (size1, count1) = calculateSizeAndCount(dir)
            val (size2, count2) = calculateSizeAndCount(bin)
            val size = size1 + size2
            val count = count1 + count2
            if (requireConfirmation) {
                stateMachine.emitLog(
                    RuntimeEvent.LogEmitted.Level.WARN,
                    TAG,
                    "待清理 RootFS: path=${dir.absolutePath} bin=${bin.absolutePath} size=$size files=$count (待二次确认)"
                )
                return@withContext Result.success(Unit)
            }
            if (dir.exists()) {
                dir.deleteRecursively()
            }
            if (bin.exists()) {
                bin.deleteRecursively()
            }
            stateMachine.emitLog(
                RuntimeEvent.LogEmitted.Level.INFO,
                TAG,
                "RootFS 已清理 path=${dir.absolutePath} bin=${bin.absolutePath}"
            )
            Result.success(Unit)
        } catch (e: Exception) {
            stateMachine.emitError(
                error = "RootFS 清理失败: ${e.message}",
                retryable = true,
                requiresUserAction = false,
                cause = e
            )
            Result.failure(e)
        }
    }

    override suspend fun info(): LinuxRootfsManager.RootfsInfo? = withContext(Dispatchers.IO) {
        val manifest = readManifest() ?: return@withContext null
        val installed = isInstalled()
        if (!installed) return@withContext null
        val version = getCurrentVersion() ?: manifest.version
        val dir = rootfsDir()
        val bin = binDir()
        val installedAt = versionFile().takeIf { it.exists() }?.lastModified() ?: 0L
        val (size1, count1) = calculateSizeAndCount(dir)
        val (size2, count2) = calculateSizeAndCount(bin)
        val components = manifest.components.map { c ->
            val target = if (c.type == TYPE_CONFIG) File(dir, c.file) else File(bin, c.file)
            LinuxRootfsManager.ComponentInfo(
                name = c.name,
                file = c.file,
                size = c.size,
                sha256 = c.sha256,
                target = c.target,
                installed = target.exists()
            )
        }
        LinuxRootfsManager.RootfsInfo(
            version = version,
            manifestVersion = manifest.version,
            installedAt = installedAt,
            sizeBytes = size1 + size2,
            fileCount = count1 + count2,
            components = components
        )
    }

    override fun minimalRootfsDir(): File = File(rootfsDir(), "minimal")

    override suspend fun ensureMinimalRootfs(): Result<Unit> = withContext(Dispatchers.IO) {
        try {
            val rootfs = minimalRootfsDir()
            rootfs.mkdirs()
            listOf(
                File(rootfs, "bin"),
                File(rootfs, "lib"),
                File(rootfs, "lib64"),
                File(rootfs, "etc"),
                File(rootfs, "tmp"),
                File(rootfs, "usr/bin"),
                File(rootfs, "usr/lib"),
                File(rootfs, "var"),
                File(rootfs, "dev"),
                File(rootfs, "proc"),
                File(rootfs, "sys")
            ).forEach { dir ->
                if (!dir.exists()) {
                    dir.mkdirs()
                }
            }

            writeEtcFiles(rootfs)

            linkBinaries(rootfs)

            stateMachine.emitLog(
                RuntimeEvent.LogEmitted.Level.INFO,
                TAG,
                "最小化 RootFS 已准备 path=${rootfs.absolutePath}"
            )
            Result.success(Unit)
        } catch (e: Exception) {
            stateMachine.emitError(
                error = "最小化 RootFS 创建失败: ${e.message}",
                retryable = true,
                requiresUserAction = false,
                cause = e
            )
            Result.failure(e)
        }
    }

    private fun writeEtcFiles(rootfs: File) {
        val etcDir = File(rootfs, "etc").apply { mkdirs() }

        val passwdFile = File(etcDir, "passwd")
        if (!passwdFile.exists()) {
            passwdFile.writeText(
                buildString {
                    appendLine("root:x:0:0:root:${ROOTFS_HOME_PATH}:/bin/sh")
                    appendLine("amitia:x:1000:1000:amitia:${ROOTFS_HOME_PATH}:/bin/sh")
                }
            )
        }

        val groupFile = File(etcDir, "group")
        if (!groupFile.exists()) {
            groupFile.writeText(
                buildString {
                    appendLine("root:x:0:")
                    appendLine("amitia:x:1000:")
                }
            )
        }

        val resolvFile = File(etcDir, "resolv.conf")
        if (!resolvFile.exists()) {
            resolvFile.writeText(
                buildString {
                    appendLine("nameserver 8.8.8.8")
                    appendLine("nameserver 1.1.1.1")
                    appendLine("options timeout:1 attempts:1")
                }
            )
        }

        val nsswitchFile = File(etcDir, "nsswitch.conf")
        if (!nsswitchFile.exists()) {
            nsswitchFile.writeText(
                buildString {
                    appendLine("passwd: files")
                    appendLine("group: files")
                    appendLine("hosts: files dns")
                    appendLine("networks: files dns")
                }
            )
        }

        val hostsFile = File(etcDir, "hosts")
        if (!hostsFile.exists()) {
            hostsFile.writeText(
                buildString {
                    appendLine("127.0.0.1 localhost")
                    appendLine("::1 localhost ip6-localhost")
                }
            )
        }

        val homeDir = File(rootfs, ROOTFS_HOME_PATH.substring(1))
        if (!homeDir.exists()) {
            homeDir.mkdirs()
        }
    }

    private fun linkBinaries(rootfs: File) {
        val binDir = File(rootfs, "bin").apply { mkdirs() }
        val runtimeBin = binDir()
        if (!runtimeBin.exists()) return

        for (binaryName in listOf(BACKEND_BINARY, QDRANT_BINARY, SURREAL_BINARY, PROOT_BINARY_NAME)) {
            val source = File(runtimeBin, binaryName)
            if (!source.exists()) continue
            val target = File(binDir, binaryName)
            if (target.exists()) continue
            try {
                source.copyTo(target, overwrite = false)
                ensureExecutable(target)
            } catch (_: Exception) {
            }
        }
    }

    private fun ensureExecutable(file: File) {
        try {
            Os.chmod(file.absolutePath, 493)
        } catch (_: Exception) {
            file.setExecutable(true, false)
        }
    }

    private fun readManifest(): RootfsManifest? {
        return try {
            context.assets.open(MANIFEST_ASSET_NAME).use { stream ->
                json.decodeFromString(RootfsManifest.serializer(), stream.bufferedReader().readText())
            }
        } catch (_: Exception) {
            null
        }
    }

    private fun cleanupResidual() {
        val rootfs = rootfsDir()
        val bin = binDir()
        if (rootfs.exists()) {
            rootfs.listFiles()?.forEach { it.deleteRecursively() }
        }
        if (bin.exists()) {
            bin.listFiles()?.forEach { it.deleteRecursively() }
        }
    }

    private fun copyAssetWithProgress(
        assetName: String,
        target: File,
        onProgress: (deltaBytes: Long, totalAssetBytes: Long) -> Unit
    ) {
        val buffer = ByteArray(BUFFER_SIZE)
        var totalCopied = 0L
        val assetTotal = try {
            context.assets.openFd(assetName).use { fd ->
                fd.length
            }
        } catch (_: Exception) {
            -1L
        }
        target.parentFile?.mkdirs()
        context.assets.open(assetName).use { input ->
            FileOutputStream(target).use { out ->
                while (true) {
                    val read = input.read(buffer)
                    if (read == -1) break
                    out.write(buffer, 0, read)
                    totalCopied += read
                    onProgress(totalCopied, assetTotal)
                }
            }
        }
    }

    private fun calculateSizeAndCount(dir: File): Pair<Long, Int> {
        if (!dir.exists()) return 0L to 0
        var size = 0L
        var count = 0
        dir.walkTopDown().forEach { f ->
            if (f.isFile) {
                size += f.length()
                count++
            }
        }
        return size to count
    }

    companion object {
        private const val TAG = "RootfsManager"
        private const val VERSION_FILE_NAME = ".amitia-rootfs-version"
        private const val MANIFEST_FILE_NAME = "manifest.json"
        private const val MANIFEST_ASSET_NAME = "rootfs-manifest.json"
        private const val TYPE_CONFIG = "config"
        private const val BUFFER_SIZE = 64 * 1024
        private const val ROOTFS_HOME_PATH = "/home/amitia"
        private const val BACKEND_BINARY = "amitia-backend-arm64"
        private const val QDRANT_BINARY = "qdrant_linux_aarch64"
        private const val SURREAL_BINARY = "surreal_linux_aarch64"
        private const val PROOT_BINARY_NAME = "proot_linux_aarch64"
    }
}
