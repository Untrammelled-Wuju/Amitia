package com.amitia.amitia_app.runtime.install.internal

import com.amitia.amitia_app.runtime.abi.RuntimeAbiGate
import com.amitia.amitia_app.runtime.install.ActiveRuntimeManager
import com.amitia.amitia_app.runtime.install.ActiveRuntimeResult
import com.amitia.amitia_app.runtime.install.DefaultInstallReceiptStore
import com.amitia.amitia_app.runtime.install.DefaultInstalledRuntimeVerifier
import com.amitia.amitia_app.runtime.install.DefaultRootfsManager
import com.amitia.amitia_app.runtime.install.DefaultSafeArchiveExtractor
import com.amitia.amitia_app.runtime.install.InstallLock
import com.amitia.amitia_app.runtime.install.InstallLockResult
import com.amitia.amitia_app.runtime.install.InstallReceiptResult
import com.amitia.amitia_app.runtime.install.InstalledRuntimeVerificationResult
import com.amitia.amitia_app.runtime.install.InstalledRuntimeVerifier
import com.amitia.amitia_app.runtime.install.PackageVerificationResult
import com.amitia.amitia_app.runtime.install.PackageVerifier
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
import com.amitia.amitia_app.runtime.manifest.internal.InstalledTreeHasher
import java.io.File

private enum class CommitStage {
    PRE_COMMIT,
    MANIFEST_COMMITTED,
    ACTIVE_COMMITTED,
}

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
        try {
            transaction.updateStage(com.amitia.amitia_app.runtime.install.TransactionStage.CREATED)

            val verifiedPackage = when (val result = verifyPackage(request, transaction)) {
                is VerifyPackageSuccess -> result.pkg
                is VerifyPackageFailure -> return result.failure
            }

            transaction.updateStage(com.amitia.amitia_app.runtime.install.TransactionStage.PACKAGE_VERIFIED)
            transaction.setRuntimeVersion(verifiedPackage.packageIndex.runtimeVersion)
            transaction.setPackageSha256(verifiedPackage.packageSha256)

            val existingReceipt = receiptStore.load(verifiedPackage.packageIndex.runtimeVersion)
            if (existingReceipt is InstallReceiptResult.Success) {
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
            if (versionExistedBeforeInstall) {
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
            val rootfsResult = rootfsManager.prepareRootfs(
                rootfsPayloadFile = verifiedPackage.rootfsPayloadFile,
                expectedRootfsId = rootfsId,
                expectedPayloadSha256 = rootfsPayloadRef.sha256,
            )

            val rootfsInfo = when (rootfsResult) {
                is RootfsPrepareResult.Reused -> rootfsResult.info
                is RootfsPrepareResult.NewlyInstalled -> rootfsResult.info
                is RootfsPrepareResult.Conflict -> {
                    return RuntimeInstallResult.Failure(
                        code = RuntimeInstallErrorCode.ROOTFS_CONFLICT,
                        message = "rootfs conflict: existing=${rootfsResult.existingRootfsId} new=${rootfsResult.newRootfsId}",
                        phase = RuntimeInstallPhase.ROOTFS_PREPARE,
                        transactionId = transaction.transactionId,
                    )
                }
                is RootfsPrepareResult.Failure -> {
                    return RuntimeInstallResult.Failure(
                        code = rootfsResult.code,
                        message = rootfsResult.message,
                        phase = RuntimeInstallPhase.ROOTFS_PREPARE,
                        transactionId = transaction.transactionId,
                    )
                }
            }

            transaction.updateStage(com.amitia.amitia_app.runtime.install.TransactionStage.ROOTFS_PREPARED)

            val stagingDir = File(layout.stagingRoot, transaction.transactionId)
            stagingDir.mkdirs()

            val runtimeExtractResult = archiveExtractor.extractTarXz(
                tarXzFile = verifiedPackage.runtimePayloadFile,
                targetDir = stagingDir,
                rootBoundary = stagingDir.absolutePath,
            )

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

            transaction.updateStage(com.amitia.amitia_app.runtime.install.TransactionStage.RUNTIME_VERIFIED)

            val versionDir = layout.runtimeVersionRoot(verifiedPackage.packageIndex.runtimeVersion)
            if (versionDir.exists()) {
                cleanupStaging(stagingDir)
                return RuntimeInstallResult.Failure(
                    code = RuntimeInstallErrorCode.RUNTIME_VERSION_CONFLICT,
                    message = "version directory appeared during installation",
                    phase = RuntimeInstallPhase.PUBLISH,
                    transactionId = transaction.transactionId,
                )
            }

            versionDir.parentFile?.mkdirs()
            if (!stagingDir.renameTo(versionDir)) {
                stagingDir.copyRecursively(versionDir, overwrite = true)
                stagingDir.deleteRecursively()
            }

            transaction.setTargetVersionDir(versionDir.absolutePath)
            transaction.updateStage(com.amitia.amitia_app.runtime.install.TransactionStage.PUBLISHED)

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

            val finalVerification = (finalVerifyResult as InstalledRuntimeVerificationResult.Success).verification
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
                runtimePayloadSha256 = verifiedPackage.packageIndex.runtimePayload.sha256,
                runtimeRootTreeSha256 = finalVerification.runtimeRootTreeSha256,
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
            } catch (_: Exception) {
            }

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
        } catch (_: Exception) {
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
        payloads.add(
            RuntimeManifestPayload(
                id = "runtime",
                role = "runtime",
                sha256 = index.runtimePayload.sha256,
                size = index.runtimePayload.size,
            )
        )
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
        return pkg.componentLock.components.map { comp ->
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

    private fun cleanupVersionDir(versionDir: File) {
        try {
            if (versionDir.exists()) {
                versionDir.deleteRecursively()
            }
        } catch (_: Exception) {
        }
    }

    private fun computeRootfsId(rootfsPayloadRef: com.amitia.amitia_app.runtime.install.PayloadRef): String {
        return rootfsPayloadRef.sha256.substring(0, 16)
    }

    private sealed interface VerifyPackageResult
    private data class VerifyPackageSuccess(val pkg: VerifiedPackage) : VerifyPackageResult
    private data class VerifyPackageFailure(val failure: RuntimeInstallResult.Failure) : VerifyPackageResult
}
