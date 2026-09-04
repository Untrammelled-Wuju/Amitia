package com.amitia.amitia_app.runtime.connection

import com.amitia.amitia_app.runtime.connection.internal.RuntimeCredentialResolver
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File
import java.nio.file.Files

class RuntimeCredentialResolverTest {
    private val resolver = RuntimeCredentialResolver()

    @Test
    fun `resolves valid token`() {
        val root = makeDataRoot("a".repeat(32))
        val result = resolver.resolve(root)
        assertTrue(result.isSuccess)
    }

    @Test
    fun `rejects short token`() {
        val root = makeDataRoot("short")
        val result = resolver.resolve(root)
        assertFalse(result.isSuccess)
    }

    @Test
    fun `rejects empty token`() {
        val root = makeDataRoot("")
        val result = resolver.resolve(root)
        assertFalse(result.isSuccess)
    }

    @Test
    fun `rejects missing file`() {
        val tmp = Files.createTempDirectory("amitia-no-file").toFile()
        val result = resolver.resolve(tmp.absolutePath)
        assertFalse(result.isSuccess)
    }

    @Test
    fun `trims token with surrounding whitespace`() {
        val root = makeDataRoot("  ${"b".repeat(32)}  \n")
        val result = resolver.resolve(root)
        assertTrue(result.isSuccess)
    }

    @Test
    fun `rejects token containing NUL`() {
        val root = makeDataRoot("${"c".repeat(16)}${"\u0000"}${"c".repeat(16)}")
        val result = resolver.resolve(root)
        assertFalse(result.isSuccess)
    }

    @Test
    fun `rejects token containing CR`() {
        val root = makeDataRoot("${"d".repeat(16)}\r${"d".repeat(16)}")
        val result = resolver.resolve(root)
        assertFalse(result.isSuccess)
    }

    private fun makeDataRoot(token: String): String {
        val tmp = Files.createTempDirectory("amitia-cred-").toFile()
        val securityDir = File(tmp, "security")
        securityDir.mkdirs()
        File(securityDir, "local-token").writeText(token)
        return tmp.absolutePath
    }
}
