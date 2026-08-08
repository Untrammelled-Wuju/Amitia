package com.amitia.amitia_app.runtime.install.internal

import com.amitia.amitia_app.runtime.install.DefaultInstalledRuntimeVerifier
import com.amitia.amitia_app.runtime.install.InstalledRuntimeVerificationResult
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

class DefaultInstalledRuntimeVerifierTest {

    @get:Rule
    val tempFolder = TemporaryFolder()

    private fun createVerifier(): DefaultInstalledRuntimeVerifier {
        return DefaultInstalledRuntimeVerifier(treeHasher = { "fixed-tree-hash-for-test" })
    }

    @Test
    fun verify_fails_whenDirectoryMissing() {
        val missingDir = File(tempFolder.root, "nonexistent")
        val verifier = createVerifier()

        val result = verifier.verify(missingDir)

        assertTrue(result is InstalledRuntimeVerificationResult.Failure)
        assertEquals(
            com.amitia.amitia_app.runtime.install.RuntimeInstallErrorCode.RUNTIME_VERIFY_FAILED,
            (result as InstalledRuntimeVerificationResult.Failure).code
        )
    }

    @Test
    fun verify_fails_whenBackendMissing() {
        val runtimeRoot = tempFolder.newFolder("runtime-no-backend")
        createMinimalRuntimeStructure(runtimeRoot, includeBackend = false)
        val verifier = createVerifier()

        val result = verifier.verify(runtimeRoot)

        assertTrue(result is InstalledRuntimeVerificationResult.Failure)
    }

    @Test
    fun verify_fails_whenNodeMissing() {
        val runtimeRoot = tempFolder.newFolder("runtime-no-node")
        createMinimalRuntimeStructure(runtimeRoot, includeNode = false)
        val verifier = createVerifier()

        val result = verifier.verify(runtimeRoot)

        assertTrue(result is InstalledRuntimeVerificationResult.Failure)
    }

    @Test
    fun verify_fails_whenNodeScriptsMissing() {
        val runtimeRoot = tempFolder.newFolder("runtime-no-scripts")
        createMinimalRuntimeStructure(runtimeRoot, includeScripts = false)
        val verifier = createVerifier()

        val result = verifier.verify(runtimeRoot)

        assertTrue(result is InstalledRuntimeVerificationResult.Failure)
    }

    @Test
    fun verify_fails_whenQdrantMissing() {
        val runtimeRoot = tempFolder.newFolder("runtime-no-qdrant")
        createMinimalRuntimeStructure(runtimeRoot, includeQdrant = false)
        val verifier = createVerifier()

        val result = verifier.verify(runtimeRoot)

        assertTrue(result is InstalledRuntimeVerificationResult.Failure)
    }

    @Test
    fun verify_fails_whenPluginHostMissing() {
        val runtimeRoot = tempFolder.newFolder("runtime-no-plugin-host")
        createMinimalRuntimeStructure(runtimeRoot, includePluginHost = false)
        val verifier = createVerifier()

        val result = verifier.verify(runtimeRoot)

        assertTrue(result is InstalledRuntimeVerificationResult.Failure)
    }

    @Test
    fun verify_fails_whenTaskHostMissing() {
        val runtimeRoot = tempFolder.newFolder("runtime-no-task-host")
        createMinimalRuntimeStructure(runtimeRoot, includeTaskHost = false)
        val verifier = createVerifier()

        val result = verifier.verify(runtimeRoot)

        assertTrue(result is InstalledRuntimeVerificationResult.Failure)
    }

    @Test
    fun verify_fails_whenGuestLayoutMissing() {
        val runtimeRoot = tempFolder.newFolder("runtime-no-guest-layout")
        createMinimalRuntimeStructure(runtimeRoot, includeGuestLayout = false)
        val verifier = createVerifier()

        val result = verifier.verify(runtimeRoot)

        assertTrue(result is InstalledRuntimeVerificationResult.Failure)
    }

    @Test
    fun verify_fails_whenMountContractMissing() {
        val runtimeRoot = tempFolder.newFolder("runtime-no-mount-contract")
        createMinimalRuntimeStructure(runtimeRoot, includeMountContract = false)
        val verifier = createVerifier()

        val result = verifier.verify(runtimeRoot)

        assertTrue(result is InstalledRuntimeVerificationResult.Failure)
    }

    @Test
    fun verify_fails_whenMutableDirPresent() {
        val runtimeRoot = tempFolder.newFolder("runtime-mutable-dir")
        createMinimalRuntimeStructure(runtimeRoot)
        File(runtimeRoot, "data").mkdirs()

        val verifier = createVerifier()
        val result = verifier.verify(runtimeRoot)

        assertTrue(result is InstalledRuntimeVerificationResult.Failure)
        assertTrue(
            (result as InstalledRuntimeVerificationResult.Failure).message.contains("mutable")
        )
    }

    @Test
    fun verify_succeeds_withCompleteStructure() {
        val runtimeRoot = tempFolder.newFolder("runtime-complete")
        createMinimalRuntimeStructure(runtimeRoot)
        val verifier = createVerifier()

        val result = verifier.verify(runtimeRoot)

        assertTrue(result is InstalledRuntimeVerificationResult.Success)
        val verification = (result as InstalledRuntimeVerificationResult.Success).verification
        assertTrue(verification.backendPresent)
        assertTrue(verification.nodePresent)
        assertTrue(verification.npmPresent)
        assertTrue(verification.npxPresent)
        assertTrue(verification.qdrantPresent)
        assertTrue(verification.pluginHostPresent)
        assertTrue(verification.taskHostPresent)
        assertTrue(verification.nodeScriptsPresent)
        assertTrue(verification.guestLayoutPresent)
        assertTrue(verification.mountContractPresent)
        assertFalse(verification.hasInvalidMutableDirs)
        assertEquals("fixed-tree-hash-for-test", verification.runtimeRootTreeSha256)
    }

    private fun createMinimalRuntimeStructure(
        root: File,
        includeBackend: Boolean = true,
        includeNode: Boolean = true,
        includeScripts: Boolean = true,
        includeQdrant: Boolean = true,
        includePluginHost: Boolean = true,
        includeTaskHost: Boolean = true,
        includeGuestLayout: Boolean = true,
        includeMountContract: Boolean = true,
    ) {
        if (includeBackend) {
            File(root, "backend").mkdirs()
            File(root, "backend/amitia-server").writeText("dummy backend binary")
        }
        if (includeNode) {
            File(root, "node/bin").mkdirs()
            File(root, "node/bin/node").writeText("dummy node binary")
            File(root, "node/lib/node_modules/npm/bin").mkdirs()
            File(root, "node/lib/node_modules/npm/bin/npm-cli.js").writeText("npm cli")
            File(root, "node/lib/node_modules/npm/bin/npx-cli.js").writeText("npx cli")
        }
        if (includeScripts) {
            File(root, "scripts/node").mkdirs()
            File(root, "scripts/node/amitia-node-prepare.sh").writeText("#!/bin/bash")
            File(root, "scripts/node/amitia-node-probe.sh").writeText("#!/bin/bash")
        }
        if (includeQdrant) {
            File(root, "qdrant/bin").mkdirs()
            File(root, "qdrant/bin/qdrant").writeText("dummy qdrant binary")
        }
        if (includePluginHost) {
            File(root, "plugin-host/dist").mkdirs()
            File(root, "plugin-host/dist/index.js").writeText("plugin host")
        }
        if (includeTaskHost) {
            File(root, "task-host/dist").mkdirs()
            File(root, "task-host/dist/index.js").writeText("task host")
        }
        if (includeGuestLayout) {
            File(root, "manifest").mkdirs()
            File(root, "manifest/guest-layout.json").writeText("{\"root\":\"/opt/amitia\"}")
        }
        if (includeMountContract) {
            File(root, "manifest").mkdirs()
            File(root, "manifest/mount-contract.json").writeText("{\"binds\":[]}")
        }
    }
}
