package com.amitia.amitia_app.runtime.manifest.internal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

class InstalledFileHasherTest {

    @Test
    fun sha256_emptyString_knownHash() {
        val hash = InstalledFileHasher.sha256String("")
        assertEquals("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", hash)
    }

    @Test
    fun sha256_knownValue() {
        val hash = InstalledFileHasher.sha256String("hello")
        assertEquals("2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", hash)
    }

    @Test
    fun sha256File_matchesStringHash() {
        val tmp = FileTempFile("test", ".txt", "hello")
        try {
            val fileHash = InstalledFileHasher.sha256(tmp)
            val stringHash = InstalledFileHasher.sha256String("hello")
            assertEquals(stringHash, fileHash)
        } finally {
            tmp.delete()
        }
    }

    private fun FileTempFile(name: String, suffix: String, content: String): File {
        val tmp = File.createTempFile(name, suffix)
        tmp.writeText(content, Charsets.UTF_8)
        return tmp
    }
}
