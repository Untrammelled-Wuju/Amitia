package com.amitia.runtime.extension.security

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test

class IntegrityVerifierTest {

    private val verifier = IntegrityVerifier()

    @Test
    fun computeFileHash_returnsCorrectSha256Hash() {
        val data = "hello".toByteArray(Charsets.UTF_8)
        val hash = verifier.computeFileHash(data)
        assertEquals("2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", hash)
    }

    @Test
    fun computeTreeHash_sortsByPathBeforeHashing() {
        val helloHash = verifier.computeFileHash("hello".toByteArray(Charsets.UTF_8))
        val worldHash = verifier.computeFileHash("world".toByteArray(Charsets.UTF_8))
        val filesReversed = listOf(
            FileEntry("b.txt", 5, worldHash),
            FileEntry("a.txt", 5, helloHash)
        )
        val filesSorted = listOf(
            FileEntry("a.txt", 5, helloHash),
            FileEntry("b.txt", 5, worldHash)
        )
        val hashReversed = verifier.computeTreeHash(filesReversed)
        val hashSorted = verifier.computeTreeHash(filesSorted)
        assertEquals(hashSorted, hashReversed)
        assertTrue(hashSorted.isNotEmpty())
    }

    @Test
    fun computeManifestHash_returnsCorrectHash() {
        val manifestRaw = "{\"version\":\"1.0\"}"
        val expected = verifier.computeFileHash(manifestRaw.toByteArray(Charsets.UTF_8))
        val actual = verifier.computeManifestHash(manifestRaw)
        assertEquals(expected, actual)
    }

    @Test
    fun computeArchiveHash_returnsSha256PrefixedHash() {
        val raw = "archive".toByteArray(Charsets.UTF_8)
        val hash = verifier.computeArchiveHash(raw)
        assertTrue(hash.startsWith("sha256:"))
        val hexPart = hash.removePrefix("sha256:")
        assertEquals(64, hexPart.length)
        assertEquals(verifier.computeFileHash(raw), hexPart)
    }

    @Test
    fun verifyIntegrity_normalCase_passes() {
        val dataA = "hello".toByteArray(Charsets.UTF_8)
        val dataB = "world".toByteArray(Charsets.UTF_8)
        val hashA = verifier.computeFileHash(dataA)
        val hashB = verifier.computeFileHash(dataB)
        val packageFiles = mapOf(
            "a.txt" to dataA,
            "b.txt" to dataB
        )
        val integrityFiles = IntegrityFilesDoc(
            files = mapOf(
                "a.txt" to FileEntry("a.txt", dataA.size.toLong(), hashA),
                "b.txt" to FileEntry("b.txt", dataB.size.toLong(), hashB)
            )
        )
        val treeEntries = listOf(
            FileEntry("a.txt", dataA.size.toLong(), hashA),
            FileEntry("b.txt", dataB.size.toLong(), hashB)
        )
        val treeHash = verifier.computeTreeHash(treeEntries)
        val integrityTree = IntegrityTreeDoc(treeHash = treeHash)

        val result = verifier.verifyIntegrity(packageFiles, integrityFiles, integrityTree)

        assertEquals(2, result.size)
        assertEquals("a.txt", result[0].path)
        assertEquals(dataA.size.toLong(), result[0].size)
        assertEquals(hashA, result[0].hash)
        assertEquals("b.txt", result[1].path)
        assertEquals(hashB, result[1].hash)
    }

    @Test
    fun verifyIntegrity_fileHashMismatch_throwsIntegrityMismatchException() {
        val dataA = "hello".toByteArray(Charsets.UTF_8)
        val packageFiles = mapOf("a.txt" to dataA)
        val integrityFiles = IntegrityFilesDoc(
            files = mapOf("a.txt" to FileEntry("a.txt", dataA.size.toLong(), "0".repeat(64)))
        )
        val integrityTree = IntegrityTreeDoc(treeHash = "")

        try {
            verifier.verifyIntegrity(packageFiles, integrityFiles, integrityTree)
            fail("Expected IntegrityMismatchException")
        } catch (e: IntegrityMismatchException) {
            assertTrue(e.message!!.contains("hash mismatch"))
        }
    }

    @Test
    fun verifyIntegrity_fileSizeMismatch_throwsIntegrityMismatchException() {
        val dataA = "hello".toByteArray(Charsets.UTF_8)
        val hashA = verifier.computeFileHash(dataA)
        val packageFiles = mapOf("a.txt" to dataA)
        val integrityFiles = IntegrityFilesDoc(
            files = mapOf("a.txt" to FileEntry("a.txt", 999L, hashA))
        )
        val integrityTree = IntegrityTreeDoc(treeHash = "")

        try {
            verifier.verifyIntegrity(packageFiles, integrityFiles, integrityTree)
            fail("Expected IntegrityMismatchException")
        } catch (e: IntegrityMismatchException) {
            assertTrue(e.message!!.contains("size mismatch"))
        }
    }

    @Test
    fun verifyIntegrity_missingIntegrityFiles_throwsIntegrityMissingException() {
        val packageFiles = mapOf("a.txt" to "hello".toByteArray(Charsets.UTF_8))
        try {
            verifier.verifyIntegrity(packageFiles, null, IntegrityTreeDoc(treeHash = ""))
            fail("Expected IntegrityMissingException")
        } catch (e: IntegrityMissingException) {
            assertTrue(e.message!!.contains("files.json"))
        }
    }

    @Test
    fun verifyIntegrity_missingIntegrityTree_throwsIntegrityMissingException() {
        val dataA = "hello".toByteArray(Charsets.UTF_8)
        val hashA = verifier.computeFileHash(dataA)
        val packageFiles = mapOf("a.txt" to dataA)
        val integrityFiles = IntegrityFilesDoc(
            files = mapOf("a.txt" to FileEntry("a.txt", dataA.size.toLong(), hashA))
        )
        try {
            verifier.verifyIntegrity(packageFiles, integrityFiles, null)
            fail("Expected IntegrityMissingException")
        } catch (e: IntegrityMissingException) {
            assertTrue(e.message!!.contains("content-tree.json"))
        }
    }

    @Test
    fun verifyIntegrity_treeHashMismatch_throwsIntegrityMismatchException() {
        val dataA = "hello".toByteArray(Charsets.UTF_8)
        val hashA = verifier.computeFileHash(dataA)
        val packageFiles = mapOf("a.txt" to dataA)
        val integrityFiles = IntegrityFilesDoc(
            files = mapOf("a.txt" to FileEntry("a.txt", dataA.size.toLong(), hashA))
        )
        val integrityTree = IntegrityTreeDoc(treeHash = "0".repeat(64))

        try {
            verifier.verifyIntegrity(packageFiles, integrityFiles, integrityTree)
            fail("Expected IntegrityMismatchException")
        } catch (e: IntegrityMismatchException) {
            assertTrue(e.message!!.contains("tree hash mismatch"))
        }
    }

    @Test
    fun verifyIntegrity_fileNotInManifest_throwsIntegrityMismatchException() {
        val dataA = "hello".toByteArray(Charsets.UTF_8)
        val packageFiles = mapOf("a.txt" to dataA)
        val integrityFiles = IntegrityFilesDoc(files = emptyMap())
        val integrityTree = IntegrityTreeDoc(treeHash = "")

        try {
            verifier.verifyIntegrity(packageFiles, integrityFiles, integrityTree)
            fail("Expected IntegrityMismatchException")
        } catch (e: IntegrityMismatchException) {
            assertTrue(e.message!!.contains("not in integrity manifest"))
        }
    }

    @Test
    fun verifyManifestContentTreeHash_matchingHash_passes() {
        val treeHash = "abc123def456"
        verifier.verifyManifestContentTreeHash(treeHash, treeHash)
    }

    @Test
    fun verifyManifestContentTreeHash_mismatch_throwsException() {
        try {
            verifier.verifyManifestContentTreeHash("abc", "def")
            fail("Expected IntegrityMismatchException")
        } catch (e: IntegrityMismatchException) {
            assertTrue(e.message!!.contains("manifest content tree hash mismatch"))
        }
    }

    @Test
    fun verifyManifestContentTreeHash_nullHash_skips() {
        verifier.verifyManifestContentTreeHash(null, "abc")
    }

    @Test
    fun verifyManifestContentTreeHash_emptyHash_skips() {
        verifier.verifyManifestContentTreeHash("", "abc")
    }
}
