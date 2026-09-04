package com.amitia.amitia_app.runtime.manifest.internal

import java.io.File
import java.security.MessageDigest

internal object InstalledFileHasher {

    fun sha256(file: File): String {
        val digest = MessageDigest.getInstance("SHA-256")
        file.inputStream().use { input ->
            val buffer = ByteArray(8192)
            while (true) {
                val n = input.read(buffer)
                if (n <= 0) break
                digest.update(buffer, 0, n)
            }
        }
        return digest.digest().toLowerHex()
    }

    fun sha256String(content: String): String {
        val digest = MessageDigest.getInstance("SHA-256")
        digest.update(content.toByteArray(Charsets.UTF_8))
        return digest.digest().toLowerHex()
    }

    private fun ByteArray.toLowerHex(): String {
        val sb = StringBuilder(size * 2)
        for (b in this) {
            val v = b.toInt() and 0xFF
            sb.append(HEX[v ushr 4])
            sb.append(HEX[v and 0x0F])
        }
        return sb.toString()
    }

    private val HEX = "0123456789abcdef".toCharArray()
}
