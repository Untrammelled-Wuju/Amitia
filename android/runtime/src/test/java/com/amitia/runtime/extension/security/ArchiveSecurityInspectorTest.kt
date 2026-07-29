package com.amitia.runtime.extension.security

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Assert.assertThrows
import org.junit.Test
import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
import java.util.zip.ZipEntry
import java.util.zip.ZipOutputStream

class ArchiveSecurityInspectorTest {

    private fun createZip(vararg entries: Pair<String, ByteArray>): ByteArray {
        val baos = ByteArrayOutputStream()
        ZipOutputStream(baos).use { zos ->
            entries.forEach { (name, data) ->
                zos.putNextEntry(ZipEntry(name))
                zos.write(data)
                zos.closeEntry()
            }
        }
        return baos.toByteArray()
    }

    private fun inspect(
        zip: ByteArray,
        policy: ArchivePolicy = ArchivePolicy.default()
    ): Pair<Map<String, ByteArray>, ArchiveSecurityInspector.InspectionSummary> {
        val inspector = ArchiveSecurityInspector(policy)
        return inspector.inspectAndExtract(ByteArrayInputStream(zip))
    }

    @Test
    fun inspectAndExtract_extractsValidArchive() {
        val zip = createZip(
            "manifest.json" to "{}".toByteArray(Charsets.UTF_8),
            "modules/main/index.js" to "console.log(1);".toByteArray(Charsets.UTF_8),
            "integrity/files.json" to "{}".toByteArray(Charsets.UTF_8),
            "integrity/content-tree.json" to "{}".toByteArray(Charsets.UTF_8)
        )
        val (entries, summary) = inspect(zip)
        assertEquals(4, entries.size)
        assertTrue(entries.containsKey("manifest.json"))
        assertTrue(entries.containsKey("modules/main/index.js"))
        assertTrue(entries.containsKey("integrity/files.json"))
        assertTrue(entries.containsKey("integrity/content-tree.json"))
        assertEquals(4, summary.entryCount)
    }

    @Test
    fun inspectAndExtract_rejectsExcessiveEntryCount() {
        val policy = ArchivePolicy.default().copy(maxEntryCount = 2)
        val zip = createZip(
            "manifest.json" to "{}".toByteArray(Charsets.UTF_8),
            "modules/main/index.js" to "console.log(1);".toByteArray(Charsets.UTF_8),
            "modules/util.js" to "export const x = 1;".toByteArray(Charsets.UTF_8)
        )
        assertThrows(EntryCountExceededException::class.java) {
            inspect(zip, policy)
        }
    }

    @Test
    fun inspectAndExtract_rejectsCaseInsensitiveDuplicate() {
        val zip = createZip(
            "modules/Main/index.js" to "console.log(1);".toByteArray(Charsets.UTF_8),
            "modules/main/index.js" to "console.log(2);".toByteArray(Charsets.UTF_8)
        )
        assertThrows(DuplicatePathException::class.java) {
            inspect(zip)
        }
    }

    @Test
    fun inspectAndExtract_rejectsPathTraversalEntry() {
        val zip = createZip("../etc/passwd" to "secret".toByteArray(Charsets.UTF_8))
        assertThrows(PathTraversalException::class.java) {
            inspect(zip)
        }
    }

    @Test
    fun inspectAndExtract_rejectsUnknownRootEntry() {
        val zip = createZip("unknown/file.txt" to "hello".toByteArray(Charsets.UTF_8))
        assertThrows(InvalidStructureException::class.java) {
            inspect(zip)
        }
    }

    @Test
    fun inspectAndExtract_rejectsForbiddenExecutableExtension() {
        val zip = createZip("modules/evil.exe" to "fake exe".toByteArray(Charsets.UTF_8))
        assertThrows(ForbiddenFileTypeException::class.java) {
            inspect(zip)
        }
    }

    @Test
    fun inspectAndExtract_rejectsExecutableMagicBytes() {
        val mzContent = byteArrayOf('M'.code.toByte(), 'Z'.code.toByte(), 0x90.toByte(), 0x00)
        val zip = createZip("modules/payload.bin" to mzContent)
        assertThrows(ForbiddenFileTypeException::class.java) {
            inspect(zip)
        }
    }
}
