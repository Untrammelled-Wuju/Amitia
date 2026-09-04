package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.proot.ProotError
import com.amitia.amitia_app.runtime.proot.ProotErrorCode
import java.io.File
import java.io.FileInputStream

internal class ProotElfInspector {
    fun inspect(file: File): ProotError? {
        if (!file.exists()) return ProotError.of(ProotErrorCode.BINARY_NOT_FOUND, "file not found")
        if (!file.isFile) return ProotError.of(ProotErrorCode.BINARY_NOT_FILE, "not a file")
        if (!file.canRead()) return ProotError.of(ProotErrorCode.BINARY_NOT_READABLE, "not readable")
        try {
            FileInputStream(file).use { fis ->
                val header = ByteArray(64)
                if (fis.read(header) < 64) return ProotError.of(ProotErrorCode.ELF_INVALID, "too small")
                if (header[0] != 0x7F.toByte() || header[1] != 'E'.code.toByte() || header[2] != 'L'.code.toByte() || header[3] != 'F'.code.toByte())
                    return ProotError.of(ProotErrorCode.ELF_INVALID, "invalid magic")
                if (header[4].toInt() and 0xFF != 2) return ProotError.of(ProotErrorCode.ELF_CLASS_UNSUPPORTED, "not 64-bit")
                if (header[5].toInt() and 0xFF != 1) return ProotError.of(ProotErrorCode.ELF_ENDIAN_UNSUPPORTED, "not little endian")
                val machine = (header[18].toInt() and 0xFF) or ((header[19].toInt() and 0xFF) shl 8)
                if (machine != 183) return ProotError.of(ProotErrorCode.ELF_ARCH_UNSUPPORTED, "not aarch64")
                var entry = 0L
                for (i in 0 until 8) entry = entry or ((header[24 + i].toLong() and 0xFF) shl (i * 8))
                if (entry == 0L) return ProotError.of(ProotErrorCode.ELF_ENTRY_INVALID, "zero entry")
                return null
            }
        } catch (e: Exception) { return ProotError.of(ProotErrorCode.ELF_INVALID, "read error: ${e.message}") }
    }
}