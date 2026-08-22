package com.amitia.amitia_app.runtime.install

import java.io.File

internal data class InstalledRuntimeVerification(
    val valid: Boolean,
    val backendPresent: Boolean,
    val nodePresent: Boolean,
    val npmPresent: Boolean,
    val npxPresent: Boolean,
    val qdrantPresent: Boolean,
    val pluginHostPresent: Boolean,
    val taskHostPresent: Boolean,
    val nodeScriptsPresent: Boolean,
    val guestLayoutPresent: Boolean,
    val mountContractPresent: Boolean,
    val hasInvalidMutableDirs: Boolean,
    val runtimeRootTreeSha256: String,
)

internal sealed interface InstalledRuntimeVerificationResult {
    data class Success(val verification: InstalledRuntimeVerification) : InstalledRuntimeVerificationResult
    data class Failure(
        val code: RuntimeInstallErrorCode,
        val message: String,
    ) : InstalledRuntimeVerificationResult
}

internal interface InstalledRuntimeVerifier {
    fun verify(runtimeRootDir: File): InstalledRuntimeVerificationResult
    fun computeTreeSha256(rootDir: File): String
}

internal class DefaultInstalledRuntimeVerifier(
    private val treeHasher: (File) -> String,
) : InstalledRuntimeVerifier {

    private val requiredFiles = listOf(
        "backend/amitia-server" to "backend",
        "node/bin/node" to "node",
        "node/lib/node_modules/npm/bin/npm-cli.js" to "npm",
        "node/lib/node_modules/npm/bin/npx-cli.js" to "npx",
        "qdrant/bin/qdrant" to "qdrant",
        "surrealdb/surreal" to "surrealdb",
        "plugin-host/dist/index.js" to "plugin-host",
        "task-host/dist/index.js" to "task-host",
        "scripts/node/amitia-node-prepare.sh" to "node-scripts-prepare",
        "scripts/node/amitia-node-probe.sh" to "node-scripts-probe",
    )

    private val requiredMetadataFiles = listOf(
        "manifest/guest-layout.json" to "guest-layout",
        "manifest/mount-contract.json" to "mount-contract",
    )

    private val forbiddenMutableDirs = listOf(
        "config", "data", "cache", "logs", "run", "workspaces",
    )

    override fun verify(runtimeRootDir: File): InstalledRuntimeVerificationResult {
        if (!runtimeRootDir.exists() || !runtimeRootDir.isDirectory) {
            return InstalledRuntimeVerificationResult.Failure(
                RuntimeInstallErrorCode.RUNTIME_VERIFY_FAILED,
                "runtime root directory does not exist: ${runtimeRootDir.absolutePath}"
            )
        }

        for ((relPath, _) in requiredFiles) {
            val f = File(runtimeRootDir, relPath)
            if (!f.exists()) {
                return InstalledRuntimeVerificationResult.Failure(
                    RuntimeInstallErrorCode.RUNTIME_VERIFY_FAILED,
                    "required file missing: $relPath"
                )
            }
        }

        for ((relPath, _) in requiredMetadataFiles) {
            val f = File(runtimeRootDir, relPath)
            if (!f.exists()) {
                return InstalledRuntimeVerificationResult.Failure(
                    RuntimeInstallErrorCode.RUNTIME_VERIFY_FAILED,
                    "required metadata file missing: $relPath"
                )
            }
        }

        val mutableDirs = mutableListOf<String>()
        val topLevelEntries = runtimeRootDir.listFiles() ?: emptyArray()
        for (entry in topLevelEntries) {
            if (entry.isDirectory && entry.name in forbiddenMutableDirs) {
                mutableDirs.add(entry.name)
            }
        }
        if (mutableDirs.isNotEmpty()) {
            return InstalledRuntimeVerificationResult.Failure(
                RuntimeInstallErrorCode.RUNTIME_VERIFY_FAILED,
                "runtime root contains mutable directories: $mutableDirs"
            )
        }

        val treeSha = computeTreeSha256(runtimeRootDir)

        return InstalledRuntimeVerificationResult.Success(
            InstalledRuntimeVerification(
                valid = true,
                backendPresent = true,
                nodePresent = true,
                npmPresent = true,
                npxPresent = true,
                qdrantPresent = true,
                pluginHostPresent = true,
                taskHostPresent = true,
                nodeScriptsPresent = true,
                guestLayoutPresent = true,
                mountContractPresent = true,
                hasInvalidMutableDirs = false,
                runtimeRootTreeSha256 = treeSha,
            )
        )
    }

    override fun computeTreeSha256(rootDir: File): String {
        return treeHasher(rootDir)
    }
}
