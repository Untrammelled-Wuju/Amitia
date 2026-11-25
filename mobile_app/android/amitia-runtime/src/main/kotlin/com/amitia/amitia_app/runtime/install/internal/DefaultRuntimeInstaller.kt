package com.amitia.amitia_app.runtime.install.internal

import com.amitia.amitia_app.runtime.abi.RuntimeAbiGate
import com.amitia.amitia_app.runtime.install.ActiveRuntimeManager
import com.amitia.amitia_app.runtime.install.ActiveRuntimeResult
import com.amitia.amitia_app.runtime.install.ComponentPayload
import com.amitia.amitia_app.runtime.install.DefaultInstallReceiptStore
import com.amitia.amitia_app.runtime.install.DefaultInstalledRuntimeVerifier
import com.amitia.amitia_app.runtime.install.DefaultRootfsManager
import com.amitia.amitia_app.runtime.install.DefaultSafeArchiveExtractor
import com.amitia.amitia_app.runtime.install.ExtractionResult
import com.amitia.amitia_app.runtime.install.InstallLock
import com.amitia.amitia_app.runtime.install.InstallLockResult
import com.amitia.amitia_app.runtime.install.InstallReceiptResult
import com.amitia.amitia_app.runtime.install.InstalledRuntimeVerificationResult
import com.amitia.amitia_app.runtime.install.InstalledRuntimeVerifier
import com.amitia.amitia_app.runtime.install.PackageVerificationResult
import com.amitia.amitia_app.runtime.install.PackageVerifier
import com.amitia.amitia_app.runtime.install.PayloadCompression
import com.amitia.amitia_app.runtime.install.RootfsPrepareResult
import com.amitia.amitia_app.runtime.install.RuntimeInstallErrorCode
import com.amitia.amitia_app.runtime.install.RuntimeInstaller
import com.amitia.amitia_app.runtime.install.RuntimeHostLayout
import com.amitia.amitia_app.runtime.install.RuntimeInstallRequest
import com.amitia.amitia_app.runtime.install.RuntimeInstallResult
import com.amitia.amitia_app.runtime.install.RuntimeInstallPhase
import com.amitia.amitia_app.runtime.install.RuntimeInstallReceipt
import com.amitia.amitia_app.runtime.install.SafeExtractResult
import com.amitia.amitia_app.runtime.install.TransactionStage
import com.amitia.amitia_app.runtime.install.VerifiedPackage
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestBuilder
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestComponent
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestPayload
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestResult
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestStore
import com.amitia.amitia_app.runtime.manifest.internal.DefaultRuntimeManifestBuilder
import com.amitia.amitia_app.runtime.manifest.internal.InstalledTreeHasher
import java.io.File
import java.nio.file.Files
import java.nio.file.LinkOption
import java.security.MessageDigest
import java.util.concurrent.Callable
import java.util.concurrent.ExecutionException
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors
import java.util.concurrent.Future

private const val COMPONENT_EXTRACT_PARALLELISM = 4

private val CLEANUP_EXECUTOR: ExecutorService = Executors.newSingleThreadExecutor { runnable ->
    Thread(runnable, "amitia-runtime-cleanup").apply { isDaemon = true }
}

private class PhaseTiming {
    private var last = System.nanoTime()
    private val phases = mutableListOf<Pair<String, Long>>()

    fun mark(name: String) {
        val now = System.nanoTime()
        phases.add(name to (now - last) / 1_000_000L)
        last = now
    }

    fun log(log: (String, Throwable?) -> Unit) {
        val text = phases.joinToString(" ") { "${it.first}=${it.second}ms" }
        log("install timing: $text total=${phases.sumOf { it.second }}ms", null)
    }
}

private enum class CommitStage {
    PRE_COMMIT,
    MANIFEST_COMMITTED,
    ACTIVE_COMMITTED,
}

private data class RepairBackup(
    val versionDir: File,
    val backupVersionDir: File?,
    val receiptFile: File,
    val receiptBytes: ByteArray?,
    val manifestBytes: ByteArray?,
    val manifestShaBytes: ByteArray?,
    val activeRuntimeBytes: ByteArray?,
    val manifestBefore: RuntimeManifestResult,
    val activeBefore: ActiveRuntimeResult,
)

internal class DefaultRuntimeInstaller(
    private val layout: RuntimeHostLayout,
    private val abiGate: RuntimeAbiGate,
    private val packageVerifier: PackageVerifier = DefaultPackageVerifier(),
    private val archiveExtractor: DefaultSafeArchiveExtractor = DefaultSafeArchiveExtractor(),
    private val rootfsManager: DefaultRootfsManager = DefaultRootfsManager(
        controlRoot = layout.controlRoot,
        extractor = archiveExtractor,
    ),
    private val runtimeVerifier: InstalledRuntimeVerifier = DefaultInstalledRuntimeVerifier(
        treeHasher = { InstalledTreeHasher.computeTreeSha256(it) }
    ),
    private val receiptStore: com.amitia.amitia_app.runtime.install.InstallReceiptStore = DefaultInstallReceiptStore(layout),
    private val manifestStore: RuntimeManifestStore,
    private val manifestBuilder: RuntimeManifestBuilder,
    private val activeRuntimeManager: ActiveRuntimeManager,
    private val installerLog: (String, Throwable?) -> Unit = { _, _ -> },
) : RuntimeInstaller {

    override fun install(request: RuntimeInstallRequest): RuntimeInstallResult {
        val abiStatus = abiGate.evaluate()
        if (abiStatus !is com.amitia.amitia_app.runtime.abi.RuntimeAbiStatus.Supported) {
            return RuntimeInstallResult.Failure(
                code = RuntimeInstallErrorCode.UNSUPPORTED_ABI,
                message = "ABI not supported",
                phase = RuntimeInstallPhase.ABI_GATE,
            )
        }

        val availableSpace = request.packageFile.parentFile?.usableSpace ?: Long.MAX_VALUE
        val requiredSpace = request.packageFile.length() * 4
        if (availableSpace < requiredSpace) {
            return RuntimeInstallResult.Failure(
                code = RuntimeInstallErrorCode.INSUFFICIENT_STORAGE,
                message = "insufficient storage: available=$availableSpace required=$requiredSpace",
                phase = RuntimeInstallPhase.SPACE_CHECK,
            )
        }

        val lockResult = InstallLock.acquire(layout)
        val lock = when (lockResult) {
            is InstallLockResult.Success -> lockResult.lock
            is InstallLockResult.Failure -> {
                return RuntimeInstallResult.Failure(
                    code = lockResult.code,
                    message = lockResult.message,
                    phase = RuntimeInstallPhase.LOCK_ACQUIRE,
                )
            }
        }

        try {
            return performInstall(request, abiStatus)
        } finally {
            lock.close()
        }
    }

    private fun performInstall(
        request: RuntimeInstallRequest,
        abiStatus: com.amitia.amitia_app.runtime.abi.RuntimeAbiStatus.Supported,
    ): RuntimeInstallResult {
        val transaction = createTransaction()
        var commitStage = CommitStage.PRE_COMMIT
        var versionExistedBeforeInstall = false
        var repairBackup: RepairBackup? = null
        var repairCompleted = false
        val timing = PhaseTiming()
        try {
            transaction.updateStage(com.amitia.amitia_app.runtime.install.TransactionStage.CREATED)

            val verifiedPackage = when (val result = verifyPackage(request, transaction)) {
                is VerifyPackageSuccess -> result.pkg
                is VerifyPackageFailure -> return result.failure
            }
            timing.mark("package-verify")

            transaction.updateStage(com.amitia.amitia_app.runtime.install.TransactionStage.PACKAGE_VERIFIED)
            transaction.setRuntimeVersion(verifiedPackage.packageIndex.runtimeVersion)
            transaction.setPackageSha256(verifiedPackage.packageSha256)

            val existingReceipt = receiptStore.load(verifiedPackage.packageIndex.runtimeVersion)
            if (existingReceipt is InstallReceiptResult.Success && !request.allowRepairExisting) {
                val receipt = existingReceipt.receipt
                if (receipt.packageSha256 == verifiedPackage.packageSha256 &&
                    receipt.runtimeRootTreeSha256.isNotEmpty()) {
                    return RuntimeInstallResult.AlreadyInstalled(
                        runtimeVersion = verifiedPackage.packageIndex.runtimeVersion,
                        packageSha256 = verifiedPackage.packageSha256,
                    )
                } else {
                    return RuntimeInstallResult.Failure(
                        code = RuntimeInstallErrorCode.RUNTIME_VERSION_CONFLICT,
                        message = "version ${verifiedPackage.packageIndex.runtimeVersion} exists with different content",
                        phase = RuntimeInstallPhase.PACKAGE_VERIFY,
                        transactionId = transaction.transactionId,
                    )
                }
            }

            val targetVersionDir = layout.runtimeVersionRoot(verifiedPackage.packageIndex.runtimeVersion)
            versionExistedBeforeInstall = targetVersionDir.exists()
            if (versionExistedBeforeInstall && !request.allowRepairExisting) {
                val existingReceiptForCompare = receiptStore.load(verifiedPackage.packageIndex.runtimeVersion)
                if (existingReceiptForCompare is InstallReceiptResult.Success &&
                    existingReceiptForCompare.receipt.packageSha256 == verifiedPackage.packageSha256) {
                    return RuntimeInstallResult.AlreadyInstalled(
                        runtimeVersion = verifiedPackage.packageIndex.runtimeVersion,
                        packageSha256 = verifiedPackage.packageSha256,
                    )
                }
                return RuntimeInstallResult.Failure(
                    code = RuntimeInstallErrorCode.RUNTIME_VERSION_CONFLICT,
                    message = "version directory already exists with different content",
                    phase = RuntimeInstallPhase.PACKAGE_VERIFY,
                    transactionId = transaction.transactionId,
                )
            }

            val rootfsPayloadRef = verifiedPackage.packageIndex.rootfsPayload
            val rootfsId = computeRootfsId(rootfsPayloadRef)
            val rootfsCompression = if (verifiedPackage.usesComponentPayloads) {
                PayloadCompression.GZIP
            } else {
                PayloadCompression.XZ
            }
            val rootfsExecutor = Executors.newSingleThreadExecutor { runnable ->
                Thread(runnable, "amitia-rootfs-prepare").apply { isDaemon = true }
            }
            val rootfsFuture = rootfsExecutor.submit(
                Callable {
                    rootfsManager.prepareRootfs(
                        rootfsPayloadFile = verifiedPackage.rootfsPayloadFile,
                        expectedRootfsId = rootfsId,
                        expectedPayloadSha256 = rootfsPayloadRef.sha256,
                        allowReplace = request.allowRepairExisting,
                        payloadCompression = rootfsCompression,
                    )
                }
            )

            val stagingDir = File(layout.stagingRoot, transaction.transactionId)
            stagingDir.mkdirs()

            val previousVersionDir = layout.runtimeVersionRoot(verifiedPackage.packageIndex.runtimeVersion)
            val previousReceipt = (receiptStore.load(verifiedPackage.packageIndex.runtimeVersion)
                as? InstallReceiptResult.Success)?.receipt

            val runtimeExtractResult = try {
                if (verifiedPackage.usesComponentPayloads) {
                    extractComponentPayloads(
                        pkg = verifiedPackage,
                        stagingDir = stagingDir,
                        previousVersionDir = previousVersionDir,
                        previousReceipt = previousReceipt,
                    )
                } else {
                    val runtimePayloadFile = verifiedPackage.runtimePayloadFile
                    if (runtimePayloadFile == null) {
                        SafeExtractResult.Failure(
                            RuntimeInstallErrorCode.PACKAGE_INVALID,
                            "missing runtime payload for package format ${verifiedPackage.packageFormatVersion}",
                        )
                    } else {
                        archiveExtractor.extractTarXz(
                            tarXzFile = runtimePayloadFile,
                            targetDir = stagingDir,
                            rootBoundary = stagingDir.absolutePath,
                        )
                    }
                }
            } catch (error: Throwable) {
                awaitRootfs(rootfsFuture, rootfsExecutor)
                throw error
            }
            timing.mark("extract")

            val rootfsResult = awaitRootfs(rootfsFuture, rootfsExecutor)
            timing.mark("rootfs")

            val rootfsInfo = when (rootfsResult) {
                is RootfsPrepareResult.Reused -> rootfsResult.info
                is RootfsPrepareResult.NewlyInstalled -> rootfsResult.info
                is RootfsPrepareResult.Replaced -> rootfsResult.info
                is RootfsPrepareResult.Conflict -> {
                    cleanupStaging(stagingDir)
                    return RuntimeInstallResult.Failure(
                        code = RuntimeInstallErrorCode.ROOTFS_CONFLICT,
                        message = "rootfs conflict: existing=${rootfsResult.existingRootfsId} new=${rootfsResult.newRootfsId}",
                        phase = RuntimeInstallPhase.ROOTFS_PREPARE,
                        transactionId = transaction.transactionId,
                    )
                }
                is RootfsPrepareResult.Failure -> {
                    cleanupStaging(stagingDir)
                    return RuntimeInstallResult.Failure(
                        code = rootfsResult.code,
                        message = rootfsResult.message,
                        phase = RuntimeInstallPhase.ROOTFS_PREPARE,
                        transactionId = transaction.transactionId,
                    )
                }
            }

            transaction.updateStage(com.amitia.amitia_app.runtime.install.TransactionStage.ROOTFS_PREPARED)

            if (runtimeExtractResult is SafeExtractResult.Failure) {
                cleanupStaging(stagingDir)
                return RuntimeInstallResult.Failure(
                    code = runtimeExtractResult.code,
                    message = runtimeExtractResult.message,
                    phase = RuntimeInstallPhase.RUNTIME_EXTRACT,
                    transactionId = transaction.transactionId,
                )
            }

            transaction.updateStage(com.amitia.amitia_app.runtime.install.TransactionStage.RUNTIME_EXTRACTED)

            val verifyResult = runtimeVerifier.verify(stagingDir)
            if (verifyResult is InstalledRuntimeVerificationResult.Failure) {
                cleanupStaging(stagingDir)
                return RuntimeInstallResult.Failure(
                    code = verifyResult.code,
                    message = verifyResult.message,
                    phase = RuntimeInstallPhase.INSTALLED_VERIFY,
                    transactionId = transaction.transactionId,
                )
            }

            timing.mark("verify-staging")
            val stagingVerification =
                (verifyResult as InstalledRuntimeVerificationResult.Success).verification
            transaction.updateStage(com.amitia.amitia_app.runtime.install.TransactionStage.RUNTIME_VERIFIED)

            val versionDir = layout.runtimeVersionRoot(verifiedPackage.packageIndex.runtimeVersion)
            if (versionDir.exists() && !request.allowRepairExisting) {
                cleanupStaging(stagingDir)
                return RuntimeInstallResult.Failure(
                    code = RuntimeInstallErrorCode.RUNTIME_VERSION_CONFLICT,
                    message = "version directory appeared during installation",
                    phase = RuntimeInstallPhase.PUBLISH,
                    transactionId = transaction.transactionId,
                )
            }

            // Capture the pre-commit authority for every install, not only
            // repair. If manifest/activation/receipt commit fails after the
            // runtime tree has been published, the operation must restore the
            // exact previous active runtime (or clean NOT_INSTALLED state)
            // instead of leaving a half-committed CORRUPTED installation.
            repairBackup = createRepairBackup(
                transactionId = transaction.transactionId,
                runtimeVersion = verifiedPackage.packageIndex.runtimeVersion,
                versionDir = versionDir,
            )

            versionDir.parentFile?.mkdirs()
            val publishedByRename = stagingDir.renameTo(versionDir)
            if (!publishedByRename) {
                stagingDir.copyRecursively(versionDir, overwrite = true)
                stagingDir.deleteRecursively()
            }
            if (!versionDir.exists()) {
                return RuntimeInstallResult.Failure(
                    code = RuntimeInstallErrorCode.INTERNAL_ERROR,
                    message = "runtime publish did not create target version directory",
                    phase = RuntimeInstallPhase.PUBLISH,
                    transactionId = transaction.transactionId,
                )
            }

            timing.mark("publish")
            transaction.setTargetVersionDir(versionDir.absolutePath)
            transaction.updateStage(com.amitia.amitia_app.runtime.install.TransactionStage.PUBLISHED)

            val finalVerification = if (publishedByRename) {
                stagingVerification
            } else {
                val finalVerifyResult = runtimeVerifier.verify(versionDir)
                if (finalVerifyResult is InstalledRuntimeVerificationResult.Failure) {
                    cleanupVersionDirIfNotCommitted(versionDir, versionExistedBeforeInstall, commitStage)
                    return RuntimeInstallResult.Failure(
                        code = finalVerifyResult.code,
                        message = finalVerifyResult.message,
                        phase = RuntimeInstallPhase.INSTALLED_VERIFY,
                        transactionId = transaction.transactionId,
                    )
                }
                (finalVerifyResult as InstalledRuntimeVerificationResult.Success).verification
            }

            timing.mark("verify-published")
            transaction.updateStage(com.amitia.amitia_app.runtime.install.TransactionStage.VERIFYING_INSTALLED)

            val manifestPayloads = buildManifestPayloads(verifiedPackage)
            val manifestComponents = buildManifestComponents(verifiedPackage)

            val manifestResult = manifestBuilder.buildFromInstalledTree(
                runtimeVersion = verifiedPackage.packageIndex.runtimeVersion,
                sourceCommit = verifiedPackage.packageIndex.sourceRevision,
                packageId = verifiedPackage.packageIndex.packageId,
                packageSha256 = verifiedPackage.packageSha256,
                rootfsId = rootfsInfo.rootfsId,
                runtimeRootTreeSha256 = finalVerification.runtimeRootTreeSha256,
                payloads = manifestPayloads,
                components = manifestComponents,
            )

            val manifest = when (manifestResult) {
                is RuntimeManifestResult.Success -> manifestResult.manifest
                is RuntimeManifestResult.Failure -> {
                    cleanupVersionDirIfNotCommitted(versionDir, versionExistedBeforeInstall, commitStage)
                    return RuntimeInstallResult.Failure(
                        code = RuntimeInstallErrorCode.RUNTIME_VERIFY_FAILED,
                        message = "failed to build manifest: ${manifestResult.error.manifestMessage}",
                        phase = RuntimeInstallPhase.INSTALLED_VERIFY,
                        transactionId = transaction.transactionId,
                    )
                }
            }

            val manifestWriteResult = manifestStore.write(manifest)
            if (manifestWriteResult is RuntimeManifestResult.Failure) {
                cleanupVersionDirIfNotCommitted(versionDir, versionExistedBeforeInstall, commitStage)
                return RuntimeInstallResult.Failure(
                    code = RuntimeInstallErrorCode.RUNTIME_VERIFY_FAILED,
                    message = "failed to write manifest: ${manifestWriteResult.error.manifestMessage}",
                    phase = RuntimeInstallPhase.INSTALLED_VERIFY,
                    transactionId = transaction.transactionId,
                )
            }

            commitStage = CommitStage.MANIFEST_COMMITTED
            transaction.updateStage(com.amitia.amitia_app.runtime.install.TransactionStage.MANIFEST_COMMITTING)

            val manifestReadBack = manifestStore.read()
            if (manifestReadBack is RuntimeManifestResult.Failure) {
                return RuntimeInstallResult.Failure(
                    code = RuntimeInstallErrorCode.RUNTIME_VERIFY_FAILED,
                    message = "manifest read-back failed: ${manifestReadBack.error.manifestMessage}",
                    phase = RuntimeInstallPhase.INSTALLED_VERIFY,
                    transactionId = transaction.transactionId,
                )
            }

            val readBackManifest = (manifestReadBack as RuntimeManifestResult.Success).manifest
            if (readBackManifest.runtimeVersion != manifest.runtimeVersion ||
                readBackManifest.packageSha256 != manifest.packageSha256
            ) {
                return RuntimeInstallResult.Failure(
                    code = RuntimeInstallErrorCode.RUNTIME_VERIFY_FAILED,
                    message = "manifest read-back mismatch",
                    phase = RuntimeInstallPhase.INSTALLED_VERIFY,
                    transactionId = transaction.transactionId,
                )
            }

            transaction.updateStage(com.amitia.amitia_app.runtime.install.TransactionStage.ACTIVATING)

            val activateResult = activeRuntimeManager.activate(manifest.runtimeVersion)
            if (activateResult is ActiveRuntimeResult.Failure) {
                return RuntimeInstallResult.Failure(
                    code = RuntimeInstallErrorCode.ACTIVE_RUNTIME_UPDATE_FAILED,
                    message = activateResult.message,
                    phase = RuntimeInstallPhase.ACTIVATE,
                    transactionId = transaction.transactionId,
                )
            }

            commitStage = CommitStage.ACTIVE_COMMITTED

            val activationReadBack = activeRuntimeManager.current()
            if (activationReadBack is ActiveRuntimeResult.Failure) {
                return RuntimeInstallResult.Failure(
                    code = RuntimeInstallErrorCode.ACTIVE_RUNTIME_UPDATE_FAILED,
                    message = "activation read-back failed: ${activationReadBack.message}",
                    phase = RuntimeInstallPhase.ACTIVATE,
                    transactionId = transaction.transactionId,
                )
            }

            val activeInfo = (activationReadBack as ActiveRuntimeResult.Active).info
            if (activeInfo.version != manifest.runtimeVersion) {
                return RuntimeInstallResult.Failure(
                    code = RuntimeInstallErrorCode.ACTIVE_RUNTIME_UPDATE_FAILED,
                    message = "activation read-back mismatch: expected=${manifest.runtimeVersion} actual=${activeInfo.version}",
                    phase = RuntimeInstallPhase.ACTIVATE,
                    transactionId = transaction.transactionId,
                )
            }

            transaction.updateStage(com.amitia.amitia_app.runtime.install.TransactionStage.ACTIVATED)

            val receipt = RuntimeInstallReceipt(
                schemaVersion = RuntimeInstallReceipt.SCHEMA_VERSION,
                runtimeVersion = verifiedPackage.packageIndex.runtimeVersion,
                packageId = verifiedPackage.packageIndex.packageId,
                packageSha256 = verifiedPackage.packageSha256,
                rootfsId = rootfsInfo.rootfsId,
                rootfsPayloadSha256 = rootfsInfo.payloadSha256,
                runtimePayloadSha256 = runtimePayloadSha256(verifiedPackage),
                runtimeRootTreeSha256 = finalVerification.runtimeRootTreeSha256,
                componentSha256 = componentSha256Map(verifiedPackage),
            )

            val receiptSaveResult = receiptStore.save(receipt)
            if (receiptSaveResult is InstallReceiptResult.Failure) {
                return RuntimeInstallResult.Failure(
                    code = receiptSaveResult.code,
                    message = receiptSaveResult.message,
                    phase = RuntimeInstallPhase.RECEIPT,
                    transactionId = transaction.transactionId,
                )
            }


            transaction.updateStage(com.amitia.amitia_app.runtime.install.TransactionStage.COMPLETED)

            val extractedMetadata = verifiedPackage.metadataDir
            extractedMetadata.deleteRecursively()
            try {
                verifiedPackage.packageFile.parentFile?.let { parent ->
                    if (parent.name.startsWith(".pkg-verify-")) {
                        parent.deleteRecursively()
                    }
                }
            } catch (originalError422: Exception) {
              originalError422.printStackTrace()

            }

            timing.mark("receipt")
            repairCompleted = true
            return RuntimeInstallResult.Success(
                runtimeVersion = receipt.runtimeVersion,
                packageSha256 = receipt.packageSha256,
                rootfsId = receipt.rootfsId,
                rootfsPayloadSha256 = receipt.rootfsPayloadSha256,
                runtimePayloadSha256 = receipt.runtimePayloadSha256,
                runtimeRootTreeSha256 = receipt.runtimeRootTreeSha256,
            )

        } catch (e: Exception) {
            transaction.updateStage(com.amitia.amitia_app.runtime.install.TransactionStage.FAILED)
            cleanupVersionDirIfNotCommitted(
                File(transaction.getJournal().targetVersionDir ?: ""),
                versionExistedBeforeInstall,
                commitStage
            )
            return RuntimeInstallResult.Failure(
                code = RuntimeInstallErrorCode.INTERNAL_ERROR,
                message = "installation failed: ${e.message}",
                phase = RuntimeInstallPhase.CLEANUP,
                transactionId = transaction.transactionId,
                cause = e,
            )
        } finally {
            val backup = repairBackup
            if (backup != null) {
                if (repairCompleted) {
                    backup.backupVersionDir?.let(::scheduleVersionDirCleanup)
                } else {
                    restoreRepairBackup(backup)
                }
            }
            rootfsManager.completePreparation(repairCompleted)?.let(::scheduleVersionDirCleanup)
            timing.mark("cleanup")
            timing.log(installerLog)
        }
    }

    private fun extractComponentPayloads(
        pkg: VerifiedPackage,
        stagingDir: File,
        previousVersionDir: File,
        previousReceipt: RuntimeInstallReceipt?,
    ): SafeExtractResult {
        val previousComponents = previousReceipt?.componentSha256.orEmpty()
        val canReuse = previousVersionDir.isDirectory && previousComponents.isNotEmpty()
        val componentRoots = pkg.packageIndex.components.associate { it.id to it.path }

        val pending = mutableListOf<ComponentPayload>()

        for (payload in pkg.componentPayloads) {
            val root = componentRoots[payload.componentId]
            val targetRoot = root?.let { File(stagingDir, it) }
            val unchanged = previousComponents[payload.componentId] == payload.sha256
            if (canReuse && unchanged && root != null && targetRoot != null) {
                if (hardlinkTree(File(previousVersionDir, root), targetRoot)) {
                    continue
                }
            }
            if (targetRoot != null && targetRoot.exists() && !targetRoot.deleteRecursively()) {
                return SafeExtractResult.Failure(
                    RuntimeInstallErrorCode.RUNTIME_EXTRACT_FAILED,
                    "cannot clear stale component directory: ${targetRoot.absolutePath}",
                )
            }
            pending.add(payload)
        }

        extractComponentsConcurrently(pending, stagingDir)?.let { return it }

        val shared = pkg.sharedPayload ?: return SafeExtractResult.Success(ExtractionResult(0, 0L))
        return archiveExtractor.extractTarGz(
            tarGzFile = shared.file,
            targetDir = stagingDir,
            rootBoundary = stagingDir.absolutePath,
        )
    }

    private fun awaitRootfs(
        future: Future<RootfsPrepareResult>,
        executor: ExecutorService,
    ): RootfsPrepareResult {
        try {
            return future.get()
        } catch (error: ExecutionException) {
            val cause = error.cause ?: error
            return RootfsPrepareResult.Failure(
                RuntimeInstallErrorCode.ROOTFS_INSTALL_FAILED,
                cause.message ?: cause.toString(),
            )
        } catch (error: InterruptedException) {
            Thread.currentThread().interrupt()
            return RootfsPrepareResult.Failure(
                RuntimeInstallErrorCode.ROOTFS_INSTALL_FAILED,
                error.message ?: "rootfs preparation interrupted",
            )
        } finally {
            executor.shutdown()
        }
    }

    private fun extractComponentsConcurrently(
        payloads: List<ComponentPayload>,
        stagingDir: File,
    ): SafeExtractResult.Failure? {
        if (payloads.isEmpty()) return null
        val ordered = payloads.sortedByDescending { it.size }
        val workers = minOf(
            COMPONENT_EXTRACT_PARALLELISM,
            ordered.size,
            Runtime.getRuntime().availableProcessors().coerceAtLeast(1),
        )
        if (workers <= 1) {
            for (payload in ordered) {
                val result = archiveExtractor.extractTarGz(
                    tarGzFile = payload.file,
                    targetDir = stagingDir,
                    rootBoundary = stagingDir.absolutePath,
                )
                if (result is SafeExtractResult.Failure) return result
            }
            return null
        }
        val pool = Executors.newFixedThreadPool(workers)
        try {
            val tasks = ordered.map { payload ->
                Callable {
                    archiveExtractor.extractTarGz(
                        tarGzFile = payload.file,
                        targetDir = stagingDir,
                        rootBoundary = stagingDir.absolutePath,
                    )
                }
            }
            val futures = pool.invokeAll(tasks)
            for ((index, future) in futures.withIndex()) {
                val result = try {
                    future.get()
                } catch (error: ExecutionException) {
                    val cause = error.cause ?: error
                    return SafeExtractResult.Failure(
                        RuntimeInstallErrorCode.RUNTIME_EXTRACT_FAILED,
                        "component extraction failed: ${ordered[index].componentId}: ${cause.message ?: cause.toString()}",
                    )
                }
                if (result is SafeExtractResult.Failure) return result
            }
            return null
        } finally {
            pool.shutdown()
        }
    }

    private fun hardlinkTree(source: File, target: File): Boolean {
        if (!source.isDirectory) return false
        return try {
            val sourceRoot = source.canonicalFile.toPath()
            var complete = true
            source.walkTopDown().forEach { file ->
                if (!complete) return@forEach
                val relative = sourceRoot.relativize(file.canonicalFile.toPath()).toString()
                val destination = if (relative.isEmpty()) target else File(target, relative)
                when {
                    Files.isSymbolicLink(file.toPath()) -> {
                        val linkTarget = Files.readSymbolicLink(file.toPath())
                        if (Files.exists(destination.toPath(), LinkOption.NOFOLLOW_LINKS)) {
                            destination.delete()
                        }
                        destination.parentFile?.mkdirs()
                        Files.createSymbolicLink(destination.toPath(), linkTarget)
                    }
                    file.isDirectory -> {
                        if (!destination.exists() && !destination.mkdirs()) {
                            complete = false
                        }
                    }
                    file.isFile -> {
                        destination.parentFile?.mkdirs()
                        if (Files.exists(destination.toPath(), LinkOption.NOFOLLOW_LINKS)) {
                            destination.delete()
                        }
                        Files.createLink(destination.toPath(), file.toPath())
                    }
                    else -> complete = false
                }
            }
            complete
        } catch (originalError: Exception) {
            originalError.printStackTrace()
            false
        }
    }

    private fun componentSha256Map(pkg: VerifiedPackage): Map<String, String> {
        val result = sortedMapOf<String, String>()
        for (payload in pkg.componentPayloads) {
            result[payload.componentId] = payload.sha256
        }
        pkg.sharedPayload?.let { result[it.componentId] = it.sha256 }
        return result
    }

    private fun runtimePayloadSha256(pkg: VerifiedPackage): String {
        val legacy = pkg.packageIndex.runtimePayload
        if (legacy != null) return legacy.sha256
        val components = componentSha256Map(pkg)
        if (components.isEmpty()) return ""
        val material = components.entries.joinToString("\n") { "${it.key}=${it.value}" }
        val digest = MessageDigest.getInstance("SHA-256")
        return digest.digest(material.toByteArray(Charsets.UTF_8)).joinToString("") { "%02x".format(it) }
    }

    private fun createRepairBackup(
        transactionId: String,
        runtimeVersion: String,
        versionDir: File,
    ): RepairBackup {
        val receiptFile = layout.installReceiptFile(runtimeVersion)
        val receiptBytes = if (receiptFile.exists()) receiptFile.readBytes() else null
        val manifestBytes = layout.runtimeManifestFile.takeIf { it.exists() }?.readBytes()
        val manifestShaBytes = layout.runtimeManifestShaFile.takeIf { it.exists() }?.readBytes()
        val activeRuntimeBytes = layout.activeRuntimeFile.takeIf { it.exists() }?.readBytes()
        val manifestBefore = manifestStore.read()
        val activeBefore = activeRuntimeManager.current()

        var backupVersionDir: File? = null
        try {
            if (versionDir.exists()) {
                val backup = File(layout.versionsRoot, ".repair-$transactionId-$runtimeVersion")
                if (backup.exists()) backup.deleteRecursively()
                backup.parentFile?.mkdirs()
                val moved = versionDir.renameTo(backup)
                if (!moved) {
                    versionDir.copyRecursively(backup, overwrite = true)
                    if (!backup.exists() || !versionDir.deleteRecursively()) {
                        backup.deleteRecursively()
                        throw IllegalStateException("failed to create repair backup for $runtimeVersion")
                    }
                }
                backupVersionDir = backup
            }
            return RepairBackup(
                versionDir = versionDir,
                backupVersionDir = backupVersionDir,
                receiptFile = receiptFile,
                receiptBytes = receiptBytes,
                manifestBytes = manifestBytes,
                manifestShaBytes = manifestShaBytes,
                activeRuntimeBytes = activeRuntimeBytes,
                manifestBefore = manifestBefore,
                activeBefore = activeBefore,
            )
        } catch (e: Exception) {
            val backup = backupVersionDir
            if (backup != null && backup.exists() && !versionDir.exists()) {
                try {
                    if (!backup.renameTo(versionDir)) {
                        backup.copyRecursively(versionDir, overwrite = true)
                        backup.deleteRecursively()
                    }
                } catch (originalError510: Exception) {
                  originalError510.printStackTrace()

                }
            }
            throw e
        }
    }

    private fun restoreRepairBackup(backup: RepairBackup) {
        try {
            if (backup.versionDir.exists()) {
                backup.versionDir.deleteRecursively()
            }
            val backupVersionDir = backup.backupVersionDir
            if (backupVersionDir != null && backupVersionDir.exists()) {
                backup.versionDir.parentFile?.mkdirs()
                if (!backupVersionDir.renameTo(backup.versionDir)) {
                    backupVersionDir.copyRecursively(backup.versionDir, overwrite = true)
                    backupVersionDir.deleteRecursively()
                }
            }
        } catch (originalError530: Exception) {
          originalError530.printStackTrace()

        }

        restoreFileBytes(layout.runtimeManifestFile, backup.manifestBytes)
        restoreFileBytes(layout.runtimeManifestShaFile, backup.manifestShaBytes)
        restoreFileBytes(layout.activeRuntimeFile, backup.activeRuntimeBytes)
        restoreFileBytes(backup.receiptFile, backup.receiptBytes)

        // Keep test/different manager implementations coherent as well as the
        // production file-backed metadata restored above.
        try {
            val previousManifest = backup.manifestBefore
            if (previousManifest is RuntimeManifestResult.Success && backup.manifestBytes == null) {
                manifestStore.write(previousManifest.manifest)
            }
        } catch (originalError545: Exception) {
          originalError545.printStackTrace()

        }
        try {
            val previousActive = backup.activeBefore
            if (previousActive is ActiveRuntimeResult.Active) {
                activeRuntimeManager.activate(previousActive.info.version)
            }
        } catch (originalError552: Exception) {
          originalError552.printStackTrace()

        }
    }

    private fun restoreFileBytes(file: File, bytes: ByteArray?) {
        try {
            if (bytes == null) {
                if (file.exists()) file.delete()
                return
            }
            file.parentFile?.mkdirs()
            val tmp = File("${file.absolutePath}.repair-restore.tmp")
            tmp.writeBytes(bytes)
            if (!tmp.renameTo(file)) {
                if (file.exists()) file.delete()
                if (!tmp.renameTo(file)) {
                    file.writeBytes(bytes)
                    tmp.delete()
                }
            }
        } catch (originalError572: Exception) {
          originalError572.printStackTrace()

        }
    }

    private fun cleanupVersionDirIfNotCommitted(
        versionDir: File,
        versionExistedBeforeInstall: Boolean,
        commitStage: CommitStage,
    ) {
        if (shouldDeletePublishedVersion(versionExistedBeforeInstall, commitStage)) {
            cleanupVersionDir(versionDir)
        }
    }

    private fun shouldDeletePublishedVersion(
        versionExistedBeforeInstall: Boolean,
        commitStage: CommitStage,
    ): Boolean {
        return !versionExistedBeforeInstall && commitStage == CommitStage.PRE_COMMIT
    }

    private fun createTransaction(): com.amitia.amitia_app.runtime.install.InstallTransaction {
        val tid = java.util.UUID.randomUUID().toString()
        val stagingDir = File(layout.stagingRoot, tid).absolutePath
        val journalFile = File(layout.transactionsRoot, "$tid.journal")
        return com.amitia.amitia_app.runtime.install.DefaultInstallTransaction(
            journalFile = journalFile,
            stagingDir = stagingDir,
        )
    }

    private fun verifyPackage(
        request: RuntimeInstallRequest,
        transaction: com.amitia.amitia_app.runtime.install.InstallTransaction,
    ): VerifyPackageResult {
        val result = packageVerifier.verify(request.packageFile, request.expectedRuntimeVersion)
        return when (result) {
            is PackageVerificationResult.Success -> VerifyPackageSuccess(result.package_)
            is PackageVerificationResult.Failure -> VerifyPackageFailure(
                RuntimeInstallResult.Failure(
                    code = result.code,
                    message = result.message,
                    phase = RuntimeInstallPhase.PACKAGE_VERIFY,
                    transactionId = transaction.transactionId,
                )
            )
        }
    }

    private fun cleanupStaging(stagingDir: File) {
        try {
            if (stagingDir.exists()) {
                stagingDir.deleteRecursively()
            }
        } catch (originalError626: Exception) {
          originalError626.printStackTrace()

        }
    }

    private fun buildManifestPayloads(pkg: VerifiedPackage): List<RuntimeManifestPayload> {
        val index = pkg.packageIndex
        val payloads = mutableListOf<RuntimeManifestPayload>()
        payloads.add(
            RuntimeManifestPayload(
                id = "rootfs",
                role = "rootfs",
                sha256 = index.rootfsPayload.sha256,
                size = index.rootfsPayload.size,
            )
        )
        val runtimePayload = index.runtimePayload
        if (runtimePayload != null) {
            payloads.add(
                RuntimeManifestPayload(
                    id = "runtime",
                    role = "runtime",
                    sha256 = runtimePayload.sha256,
                    size = runtimePayload.size,
                )
            )
        }
        for (component in index.components) {
            val payloadSha = component.payloadSha256 ?: continue
            payloads.add(
                RuntimeManifestPayload(
                    id = component.id,
                    role = "runtime-component",
                    sha256 = payloadSha,
                    size = component.payloadSize ?: 0L,
                )
            )
        }
        index.sharedPayload?.let { shared ->
            payloads.add(
                RuntimeManifestPayload(
                    id = shared.id,
                    role = "runtime-component",
                    sha256 = shared.sha256,
                    size = shared.size,
                )
            )
        }
        payloads.add(
            RuntimeManifestPayload(
                id = "sha256sums",
                role = "integrity",
                sha256 = index.sha256sums.sha256,
                size = index.sha256sums.size,
            )
        )
        index.licenses?.let { lic ->
            payloads.add(
                RuntimeManifestPayload(
                    id = "licenses",
                    role = "licenses",
                    sha256 = lic.sha256,
                    size = lic.size,
                )
            )
        }
        return payloads
    }

    private fun buildManifestComponents(pkg: VerifiedPackage): List<RuntimeManifestComponent> {
        val lockComponents = pkg.componentLock.components
        if (lockComponents.isEmpty()) {
            throw IllegalStateException(
                "manifest components empty; packageId=${pkg.packageIndex.packageId}" +
                    " componentLockRuntime=${pkg.componentLock.runtimeVersion}"
            )
        }
        return lockComponents.map { comp ->
            RuntimeManifestComponent(
                id = comp.id,
                version = comp.version,
                architecture = comp.architecture,
                root = comp.path,
                entry = null,
                sha256 = comp.sha256,
                treeSha256 = null,
                source = RuntimeManifestComponent.SOURCE_PACKAGE,
            )
        }
    }

    private fun scheduleVersionDirCleanup(versionDir: File) {
        CLEANUP_EXECUTOR.execute {
            try {
                cleanupVersionDir(versionDir)
            } catch (error: Exception) {
                installerLog(
                    "deferred version dir cleanup failed: ${versionDir.absolutePath}: ${error.message}",
                    error,
                )
            }
        }
    }

    private fun cleanupVersionDir(versionDir: File) {
        try {
            if (versionDir.exists()) {
                versionDir.deleteRecursively()
            }
        } catch (originalError697: Exception) {
          originalError697.printStackTrace()

        }
    }

    private fun computeRootfsId(rootfsPayloadRef: com.amitia.amitia_app.runtime.install.PayloadRef): String {
        return rootfsPayloadRef.sha256.substring(0, 16)
    }

    private sealed interface VerifyPackageResult
    private data class VerifyPackageSuccess(val pkg: VerifiedPackage) : VerifyPackageResult
    private data class VerifyPackageFailure(val failure: RuntimeInstallResult.Failure) : VerifyPackageResult
}
