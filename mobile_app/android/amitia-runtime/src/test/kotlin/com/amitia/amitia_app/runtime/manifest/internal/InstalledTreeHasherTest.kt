package com.amitia.amitia_app.runtime.manifest.internal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Test
import java.io.File

class InstalledTreeHasherTest {

    @Test
    fun emptyDirectory_producesDeterministicHash() {
        val dir = createTempDir("tree_empty")
        try {
            val hash1 = InstalledTreeHasher.computeTreeSha256(dir)
            val hash2 = InstalledTreeHasher.computeTreeSha256(dir)
            assertEquals(hash1, hash2)
            assertEquals(64, hash1.length)
        } finally {
            dir.deleteRecursively()
        }
    }

    @Test
    fun sameContent_sameTreeHash() {
        val dir1 = createTempDir("tree_a")
        val dir2 = createTempDir("tree_b")
        try {
            File(dir1, "file.txt").writeText("hello", Charsets.UTF_8)
            File(dir2, "file.txt").writeText("hello", Charsets.UTF_8)
            val hash1 = InstalledTreeHasher.computeTreeSha256(dir1)
            val hash2 = InstalledTreeHasher.computeTreeSha256(dir2)
            assertEquals(hash1, hash2)
        } finally {
            dir1.deleteRecursively()
            dir2.deleteRecursively()
        }
    }

    @Test
    fun differentContent_differentTreeHash() {
        val dir1 = createTempDir("tree_c")
        val dir2 = createTempDir("tree_d")
        try {
            File(dir1, "file.txt").writeText("hello1", Charsets.UTF_8)
            File(dir2, "file.txt").writeText("hello2", Charsets.UTF_8)
            val hash1 = InstalledTreeHasher.computeTreeSha256(dir1)
            val hash2 = InstalledTreeHasher.computeTreeSha256(dir2)
            assertNotEquals(hash1, hash2)
        } finally {
            dir1.deleteRecursively()
            dir2.deleteRecursively()
        }
    }

    @Test
    fun differentFileNames_differentTreeHash() {
        val dir1 = createTempDir("tree_e")
        val dir2 = createTempDir("tree_f")
        try {
            File(dir1, "a.txt").writeText("hello", Charsets.UTF_8)
            File(dir2, "b.txt").writeText("hello", Charsets.UTF_8)
            val hash1 = InstalledTreeHasher.computeTreeSha256(dir1)
            val hash2 = InstalledTreeHasher.computeTreeSha256(dir2)
            assertNotEquals(hash1, hash2)
        } finally {
            dir1.deleteRecursively()
            dir2.deleteRecursively()
        }
    }

    @Test
    fun nestedDirectories_included() {
        val dir = createTempDir("tree_g")
        try {
            val sub = File(dir, "sub")
            sub.mkdirs()
            File(sub, "inner.txt").writeText("inner", Charsets.UTF_8)
            val hash1 = InstalledTreeHasher.computeTreeSha256(dir)
            assertEquals(64, hash1.length)
        } finally {
            dir.deleteRecursively()
        }
    }

    private fun createTempDir(prefix: String): File {
        val tmp = File.createTempFile(prefix, "")
        tmp.delete()
        tmp.mkdirs()
        return tmp
    }
}
