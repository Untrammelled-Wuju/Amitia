package com.amitia.core.logging

import android.content.Context
import android.util.Log
import dagger.hilt.android.qualifiers.ApplicationContext
import java.io.File
import java.io.FileOutputStream
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.concurrent.ConcurrentLinkedQueue
import java.util.concurrent.atomic.AtomicLong
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock

@Singleton
class Logger @Inject constructor(
    @ApplicationContext private val context: Context
) {

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val writeMutex = Mutex()
    private val recentBuffer = ConcurrentLinkedQueue<String>()
    private val currentSize = AtomicLong(0L)
    private val maxBufferSize = MAX_BUFFER_ENTRIES
    private var minLevel = Level.DEBUG

    private val logDir: File by lazy {
        File(context.filesDir, "runtime/logs").apply { mkdirs() }
    }

    private val logFile: File by lazy { File(logDir, LOG_FILE_NAME) }

    private val dateFormat by lazy {
        SimpleDateFormat("yyyy-MM-dd HH:mm:ss.SSS", Locale.getDefault())
    }

    fun setMinLevel(level: Level) {
        minLevel = level
    }

    fun d(tag: String = DEFAULT_TAG, message: String) {
        if (minLevel.value > Level.DEBUG.value) return
        val sanitized = LogSanitizer.redact(message)
        Log.d(tag, sanitized)
        writeToFile("D", tag, sanitized, null)
    }

    fun i(tag: String = DEFAULT_TAG, message: String) {
        if (minLevel.value > Level.INFO.value) return
        val sanitized = LogSanitizer.redact(message)
        Log.i(tag, sanitized)
        writeToFile("I", tag, sanitized, null)
    }

    fun w(tag: String = DEFAULT_TAG, message: String, t: Throwable? = null) {
        if (minLevel.value > Level.WARN.value) return
        val sanitized = LogSanitizer.redact(message)
        if (t != null) Log.w(tag, sanitized, t) else Log.w(tag, sanitized)
        writeToFile("W", tag, sanitized, t)
    }

    fun e(tag: String = DEFAULT_TAG, message: String, t: Throwable? = null) {
        if (minLevel.value > Level.ERROR.value) return
        val sanitized = LogSanitizer.redact(message)
        if (t != null) Log.e(tag, sanitized, t) else Log.e(tag, sanitized)
        writeToFile("E", tag, sanitized, t)
    }

    fun exportDiagnostics(extraInfoProvider: (() -> String)? = null): File {
        return try {
            val timestamp = SimpleDateFormat("yyyyMMdd_HHmmss", Locale.getDefault()).format(Date())
            val exportDir = File(context.cacheDir, "diagnostics").apply { mkdirs() }
            val textFile = File(exportDir, "amitia_diagnostics_$timestamp.txt")
            val builder = StringBuilder()
            builder.appendLine("=== Amitia Diagnostics ===")
            builder.appendLine("Generated: ${Date()}")
            builder.appendLine("App: ${context.packageName}")
            builder.appendLine()
            extraInfoProvider?.let {
                builder.appendLine("=== Runtime / Config Snapshot ===")
                builder.appendLine(LogSanitizer.redact(it()))
                builder.appendLine()
            }
            builder.appendLine("=== Recent Logs ===")
            recentBuffer.forEach { builder.appendLine(it) }
            builder.appendLine()
            builder.appendLine("=== Log File Tail ===")
            readTail(logFile, 200).forEach { builder.appendLine(it) }
            textFile.writeText(builder.toString())

            val zipFile = File(exportDir, "amitia_diagnostics_$timestamp.zip")
            zipFile.delete()
            java.util.zip.ZipOutputStream(FileOutputStream(zipFile)).use { zip ->
                zip.putNextEntry(java.util.zip.ZipEntry("diagnostics.txt"))
                textFile.inputStream().use { it.copyTo(zip) }
                zip.closeEntry()
                if (logFile.exists()) {
                    zip.putNextEntry(java.util.zip.ZipEntry("amitia-android.log"))
                    logFile.inputStream().use { it.copyTo(zip) }
                    zip.closeEntry()
                }
            }
            textFile.delete()
            zipFile
        } catch (t: Throwable) {
            Log.e(DEFAULT_TAG, "exportDiagnostics failed", t)
            File(context.cacheDir, "diagnostics_failed.txt").apply {
                writeText("export failed: ${t.message}")
            }
        }
    }

    private fun writeToFile(level: String, tag: String, message: String, throwable: Throwable?) {
        val time = dateFormat.format(Date())
        val line = buildString {
            append("[$time] [$level] [$tag] $message")
            if (throwable != null) {
                append("\n")
                append(LogSanitizer.redactThrowable(throwable))
            }
        }
        recentBuffer.add(line)
        while (recentBuffer.size > maxBufferSize) recentBuffer.poll()
        scope.launch {
            writeMutex.withLock {
                runCatching {
                    if (!logFile.exists()) {
                        logFile.parentFile?.mkdirs()
                        logFile.createNewFile()
                        currentSize.set(0L)
                    }
                    if (currentSize.get() >= MAX_FILE_SIZE) {
                        val rotated = File(logFile.parentFile, "${logFile.name}.1")
                        if (rotated.exists()) rotated.delete()
                        logFile.renameTo(rotated)
                        logFile.createNewFile()
                        currentSize.set(0L)
                    }
                    val bytes = (line + System.lineSeparator()).toByteArray(Charsets.UTF_8)
                    logFile.appendBytes(bytes)
                    currentSize.addAndGet(bytes.size.toLong())
                }
            }
        }
    }

    private fun readTail(file: File, lines: Int): List<String> {
        if (!file.exists() || !file.isFile) return emptyList()
        return runCatching { file.readLines().takeLast(lines) }.getOrDefault(emptyList())
    }

    enum class Level(val value: Int) {
        VERBOSE(2), DEBUG(3), INFO(4), WARN(5), ERROR(6), ASSERT(7)
    }

    companion object {
        private const val DEFAULT_TAG = "Amitia"
        private const val LOG_FILE_NAME = "amitia-android.log"
        private const val MAX_FILE_SIZE = 5L * 1024 * 1024
        private const val MAX_BUFFER_ENTRIES = 1000
    }
}
