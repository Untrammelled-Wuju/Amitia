package com.amitia.runtime.process

import java.io.File
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.concurrent.ConcurrentHashMap
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class LogRotator @Inject constructor() {

    private val locks = ConcurrentHashMap<String, Any>()

    fun writeLine(logFile: File, line: String, maxSizeBytes: Long = DEFAULT_MAX_SIZE) {
        val key = logFile.absolutePath
        val lock = locks.computeIfAbsent(key) { Any() }
        synchronized(lock) {
            if (logFile.exists() && logFile.length() >= maxSizeBytes) {
                val rotated = File(logFile.parentFile, "${logFile.nameWithoutExtension}-${timestamp()}.log")
                if (rotated.exists()) rotated.delete()
                logFile.renameTo(rotated)
                pruneOldRotations(logFile)
            }
            logFile.parentFile?.mkdirs()
            logFile.appendText(line + System.lineSeparator())
        }
    }

    fun writeBytes(logFile: File, bytes: ByteArray, maxSizeBytes: Long = DEFAULT_MAX_SIZE) {
        val key = logFile.absolutePath
        val lock = locks.computeIfAbsent(key) { Any() }
        synchronized(lock) {
            if (logFile.exists() && logFile.length() >= maxSizeBytes) {
                val rotated = File(logFile.parentFile, "${logFile.nameWithoutExtension}-${timestamp()}.log")
                if (rotated.exists()) rotated.delete()
                logFile.renameTo(rotated)
                pruneOldRotations(logFile)
            }
            logFile.parentFile?.mkdirs()
            logFile.appendBytes(bytes)
        }
    }

    fun readTail(logFile: File, lines: Int): List<String> {
        if (!logFile.exists() || !logFile.isFile) return emptyList()
        val key = logFile.absolutePath
        val lock = locks.computeIfAbsent(key) { Any() }
        synchronized(lock) {
            return try {
                logFile.readLines().takeLast(lines)
            } catch (_: Exception) {
                emptyList()
            }
        }
    }

    fun currentSize(logFile: File): Long {
        return if (logFile.exists() && logFile.isFile) logFile.length() else 0L
    }

    fun listRotations(logFile: File): List<File> {
        val parent = logFile.parentFile ?: return emptyList()
        val baseName = logFile.nameWithoutExtension
        if (!parent.exists()) return emptyList()
        return parent.listFiles { f ->
            f.isFile && f.name.startsWith("$baseName-") && f.name.endsWith(".log")
        }?.sortedByDescending { it.lastModified() } ?: emptyList()
    }

    private fun pruneOldRotations(logFile: File) {
        val rotations = listRotations(logFile)
        if (rotations.size > MAX_ROTATIONS) {
            rotations.drop(MAX_ROTATIONS).forEach { it.delete() }
        }
    }

    private fun timestamp(): String {
        return SimpleDateFormat(TIMESTAMP_PATTERN, Locale.US).format(Date())
    }

    companion object {
        const val DEFAULT_MAX_SIZE: Long = 5L * 1024 * 1024
        const val MAX_ROTATIONS: Int = 5
        private const val TIMESTAMP_PATTERN = "yyyyMMdd-HHmmss"
    }
}
