package com.amitia.amitia_app.runtime.logging

import java.io.Closeable
import java.io.File
import java.io.FileOutputStream
import java.io.IOException
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

class DailySizeRotatingLog(
    private val directory: File,
    private val prefix: String,
    private val maxFileBytes: Long = MAX_FILE_BYTES,
    private val retentionMs: Long = RETENTION_MS,
    private val now: () -> Long = System::currentTimeMillis,
) : Closeable {
    private val lock = Any()
    private var output: FileOutputStream? = null
    private var currentFile: File? = null
    private var currentDateKey = ""
    private var currentSequence = 0
    private var currentSize = 0L

    init {
        require(prefix.isNotBlank()) { "log prefix must not be blank" }
        require(maxFileBytes > 0) { "maxFileBytes must be positive" }
        require(retentionMs > 0) { "retentionMs must be positive" }
        if (!directory.isDirectory && !directory.mkdirs() && !directory.isDirectory) {
            throw IOException("create log directory failed: ${directory.absolutePath}")
        }
        cleanupExpired()
        openForTimestamp(now())
    }

    fun write(value: String) {
        if (value.isEmpty()) return
        val bytes = value.toByteArray(Charsets.UTF_8)
        synchronized(lock) {
            val timestamp = now()
            val dateKey = dateKey(timestamp)
            if (output == null ||
                dateKey != currentDateKey ||
                currentSize > 0 && currentSize + bytes.size > maxFileBytes
            ) {
                openForTimestamp(timestamp)
            }
            val stream = output ?: throw IOException("rotating log is not open")
            try {
                stream.write(bytes)
                currentSize += bytes.size
            } catch (error: IOException) {
                throw IOException(
                    "write log failed: ${currentFile?.absolutePath ?: directory.absolutePath}",
                    error,
                )
            }
        }
    }

    override fun close() {
        synchronized(lock) {
            try {
                output?.close()
            } catch (error: IOException) {
                throw IOException("close log failed: ${currentFile?.absolutePath ?: directory.absolutePath}", error)
            } finally {
                output = null
                currentFile = null
                currentDateKey = ""
                currentSequence = 0
                currentSize = 0
            }
        }
    }

    val path: String
        get() = synchronized(lock) { currentFile?.absolutePath.orEmpty() }

    private fun openForTimestamp(timestamp: Long) {
        val dateKey = dateKey(timestamp)
        try {
            output?.close()
        } catch (error: IOException) {
            throw IOException("close log before rotation failed: ${currentFile?.absolutePath ?: directory.absolutePath}", error)
        } finally {
            output = null
        }

        var sequence = latestSequence(dateKey)
        var target = fileFor(dateKey, sequence)
        while (target.exists() && target.length() >= maxFileBytes) {
            sequence += 1
            target = fileFor(dateKey, sequence)
        }
        val stream = FileOutputStream(target, true)
        output = stream
        currentFile = target
        currentDateKey = dateKey
        currentSequence = sequence
        currentSize = target.length()
    }

    private fun latestSequence(dateKey: String): Int {
        val expression = Regex(
            "^${Regex.escape(prefix)}\\.$dateKey\\.(\\d{3})\\.log$",
        )
        var maximum = 0
        directory.listFiles()?.forEach { file ->
            if (!file.isFile) return@forEach
            val match = expression.matchEntire(file.name) ?: return@forEach
            maximum = maxOf(maximum, match.groupValues[1].toInt())
        }
        return if (maximum == 0) 1 else maximum
    }

    private fun cleanupExpired() {
        val cutoff = now() - retentionMs
        directory.listFiles()?.forEach { file ->
            if (!file.isFile || !isRetainedLogName(file.name)) return@forEach
            if (file.lastModified() >= cutoff) return@forEach
            if (!file.delete() && file.exists()) {
                throw IOException("delete expired log failed: ${file.absolutePath}")
            }
        }
    }

    private fun fileFor(dateKey: String, sequence: Int): File {
        return File(directory, "$prefix.$dateKey.${sequence.toString().padStart(3, '0')}.log")
    }

    private fun isRetainedLogName(name: String): Boolean {
        val lower = name.lowercase(Locale.US)
        if (lower.endsWith(".log") || lower.endsWith(".log.old")) return true
        val index = lower.lastIndexOf(".log.")
        if (index < 0) return false
        val suffix = lower.substring(index + ".log.".length)
        return suffix.isNotEmpty() && suffix.all(Char::isDigit)
    }

    companion object {
        const val MAX_FILE_BYTES: Long = 50L * 1024L * 1024L
        const val RETENTION_MS: Long = 30L * 24L * 60L * 60L * 1000L

        fun dateKey(timestamp: Long): String {
            return SimpleDateFormat("yyyyMMdd", Locale.US).format(Date(timestamp))
        }
    }
}
