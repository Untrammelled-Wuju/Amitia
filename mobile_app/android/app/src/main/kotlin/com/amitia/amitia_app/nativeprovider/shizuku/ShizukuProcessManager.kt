package com.amitia.amitia_app.nativeprovider.shizuku

import android.util.Base64
import org.json.JSONArray
import org.json.JSONObject
import java.io.ByteArrayOutputStream
import java.io.File
import java.io.InputStream
import java.io.OutputStream
import java.util.UUID
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicBoolean

internal class ShizukuProcessManager {

    private val processes = ConcurrentHashMap<String, ManagedProcess>()

    fun start(requestJson: String): String {
        val request = JSONObject(requestJson)
        val executable = request.optString("executable").trim()
        if (executable.isEmpty()) {
            return error("EXECUTABLE_REQUIRED", "executable is required")
        }
        val argsArray = request.optJSONArray("args") ?: JSONArray()
        val args = (0 until argsArray.length()).map { index -> argsArray.opt(index).toString() }
        val envObject = request.optJSONObject("env") ?: JSONObject()
        val env = mutableMapOf<String, String>()
        val envKeys = envObject.keys()
        while (envKeys.hasNext()) {
            val key = envKeys.next()
            env[key] = envObject.optString(key)
        }
        val workDir = request.optString("workDir").takeIf { it.isNotBlank() }
        val maxBufferBytes = request.optLong("maxBufferBytes", DEFAULT_MAX_BUFFER_BYTES)
            .coerceIn(1024L, MAX_BUFFER_BYTES)
        return try {
            val command = mutableListOf(executable)
            command.addAll(args)
            val builder = ProcessBuilder(command)
            builder.redirectErrorStream(false)
            if (env.isNotEmpty()) {
                builder.environment().putAll(env)
            }
            if (workDir != null) {
                builder.directory(File(workDir))
            }
            val process = builder.start()
            val processId = "proc-" + UUID.randomUUID().toString()
            val stdout = ProcessBuffer(maxBufferBytes)
            val stderr = ProcessBuffer(maxBufferBytes)
            val managed = ManagedProcess(process, process.outputStream, stdout, stderr)
            processes[processId] = managed
            stdout.startReader(process.inputStream)
            stderr.startReader(process.errorStream)
            JSONObject()
                .put("processId", processId)
                .put("started", true)
                .put("running", process.isAliveCompat())
                .toString()
        } catch (error: Exception) {
            error("PROCESS_START_FAILED", error.message ?: "failed to start process")
        }
    }

    fun write(requestJson: String): String {
        val request = JSONObject(requestJson)
        val managed = process(request) ?: return processNotFound()
        return try {
            val text = request.optString("text")
            val data = if (request.has("dataBase64")) {
                Base64.decode(request.optString("dataBase64"), Base64.DEFAULT)
            } else {
                text.toByteArray(Charsets.UTF_8)
            }
            managed.stdin.write(data)
            managed.stdin.flush()
            if (request.optBoolean("closeStdin", false)) {
                managed.stdin.close()
            }
            JSONObject()
                .put("success", true)
                .put("bytesWritten", data.size)
                .toString()
        } catch (error: Exception) {
            error("PROCESS_WRITE_FAILED", error.message ?: "failed to write process stdin")
        }
    }

    fun read(requestJson: String): String {
        val request = JSONObject(requestJson)
        val managed = process(request) ?: return processNotFound()
        val maxBytes = request.optLong("maxBytes", 65536L).coerceIn(1L, 8L * 1024L * 1024L)
        val waitMs = request.optLong("waitMs", 0L).coerceIn(0L, 30000L)
        val stdout = managed.stdout.read(maxBytes, waitMs)
        val stderr = managed.stderr.read(maxBytes, 0L)
        val exitCode = managed.exitCodeOrNull()
        return JSONObject()
            .put("success", true)
            .put("processId", processId(managed))
            .put("running", managed.process.isAliveCompat())
            .put("exitCode", exitCode ?: JSONObject.NULL)
            .put("stdout", stdout.toString(Charsets.UTF_8))
            .put("stderr", stderr.toString(Charsets.UTF_8))
            .put("stdoutBase64", Base64.encodeToString(stdout, Base64.NO_WRAP))
            .put("stderrBase64", Base64.encodeToString(stderr, Base64.NO_WRAP))
            .put("stdoutTruncated", managed.stdout.truncated)
            .put("stderrTruncated", managed.stderr.truncated)
            .toString()
    }

    fun wait(requestJson: String): String {
        val request = JSONObject(requestJson)
        val managed = process(request) ?: return processNotFound()
        val timeoutMs = request.optLong("timeoutMs", 30000L).coerceIn(0L, 120000L)
        return try {
            val deadline = System.currentTimeMillis() + timeoutMs
            var finished = !managed.process.isAliveCompat()
            while (!finished && System.currentTimeMillis() < deadline) {
                Thread.sleep(minOf(50L, (deadline - System.currentTimeMillis()).coerceAtLeast(1L)))
                finished = !managed.process.isAliveCompat()
            }
            if (finished) {
                managed.markFinished()
            }
            JSONObject()
                .put("success", true)
                .put("processId", processId(managed))
                .put("finished", finished)
                .put("running", managed.process.isAliveCompat())
                .put("exitCode", if (finished) managed.process.exitValue() else JSONObject.NULL)
                .put("stdout", managed.stdout.readAllUtf8())
                .put("stderr", managed.stderr.readAllUtf8())
                .toString()
        } catch (error: Exception) {
            error("PROCESS_WAIT_FAILED", error.message ?: "failed to wait for process")
        }
    }

    fun kill(requestJson: String): String {
        val request = JSONObject(requestJson)
        val processId = request.optString("processId")
        val managed = processes.remove(processId) ?: return processNotFound()
        return try {
            managed.process.destroy()
            if (request.optBoolean("force", true) && managed.process.isAliveCompat()) {
                managed.process.destroyForcibly()
            }
            managed.stdin.closeQuietly()
            managed.stdout.close()
            managed.stderr.close()
            JSONObject()
                .put("success", true)
                .put("processId", processId)
                .put("running", managed.process.isAliveCompat())
                .toString()
        } catch (error: Exception) {
            error("PROCESS_KILL_FAILED", error.message ?: "failed to kill process")
        }
    }

    fun destroyAll() {
        processes.keys.forEach { processId ->
            processes.remove(processId)?.let { managed ->
                try {
                    managed.process.destroyForcibly()
                    managed.stdin.closeQuietly()
                    managed.stdout.close()
                    managed.stderr.close()
                } catch (_: Exception) {
                }
            }
        }
    }

    private fun process(request: JSONObject): ManagedProcess? =
        processes[request.optString("processId")]

    private fun processId(managed: ManagedProcess): String =
        processes.entries.firstOrNull { it.value === managed }?.key.orEmpty()

    private fun processNotFound(): String =
        error("PROCESS_NOT_FOUND", "processId was not found")

    private fun error(code: String, message: String): String =
        JSONObject()
            .put("error", JSONObject().put("code", code).put("message", message))
            .toString()

    private data class ManagedProcess(
        val process: Process,
        val stdin: OutputStream,
        val stdout: ProcessBuffer,
        val stderr: ProcessBuffer,
    ) {
        private val finished = AtomicBoolean(false)

        fun markFinished() {
            finished.set(true)
        }

        fun exitCodeOrNull(): Int? =
            if (finished.get() || !isAlive()) {
                try {
                    process.exitValue()
                } catch (_: Exception) {
                    null
                }
            } else {
                null
            }

        private fun isAlive(): Boolean =
            try {
                process.exitValue()
                false
            } catch (_: IllegalThreadStateException) {
                true
            } catch (_: Throwable) {
                false
            }
    }

    private class ProcessBuffer(
        private val maxBytes: Long,
    ) {
        private val lock = Object()
        private val buffer = ByteArrayOutputStream()
        private var closed = false
        var truncated: Boolean = false
            private set

        fun startReader(stream: InputStream) {
            Thread {
                val chunk = ByteArray(8192)
                try {
                    while (true) {
                        val read = stream.read(chunk)
                        if (read < 0) break
                        if (read > 0) append(chunk, read)
                    }
                } catch (_: Exception) {
                } finally {
                    close()
                }
            }.apply {
                isDaemon = true
                start()
            }
        }

        fun append(bytes: ByteArray, length: Int) {
            synchronized(lock) {
                val remaining = maxBytes - buffer.size()
                if (remaining <= 0) {
                    truncated = true
                    return@synchronized
                }
                val writeLength = minOf(length.toLong(), remaining).toInt()
                buffer.write(bytes, 0, writeLength)
                if (writeLength < length) {
                    truncated = true
                }
                lock.notifyAll()
            }
        }

        fun read(maxBytes: Long, waitMs: Long): ByteArray {
            synchronized(lock) {
                if (buffer.size() == 0 && !closed && waitMs > 0) {
                    lock.wait(waitMs)
                }
                val bytes = buffer.toByteArray()
                val count = minOf(bytes.size.toLong(), maxBytes).toInt()
                val result = bytes.copyOfRange(0, count)
                val remaining = bytes.copyOfRange(count, bytes.size)
                buffer.reset()
                buffer.write(remaining)
                return result
            }
        }

        fun readAllUtf8(): String = synchronized(lock) {
            buffer.toString(Charsets.UTF_8.name())
        }

        fun close() {
            synchronized(lock) {
                closed = true
                lock.notifyAll()
            }
        }
    }

    private fun Process.isAliveCompat(): Boolean =
        try {
            exitValue()
            false
        } catch (_: IllegalThreadStateException) {
            true
        } catch (_: Throwable) {
            false
        }

    private fun OutputStream.closeQuietly() {
        try {
            close()
        } catch (_: Exception) {
        }
    }

    companion object {
        private const val DEFAULT_MAX_BUFFER_BYTES = 1024L * 1024L
        private const val MAX_BUFFER_BYTES = 64L * 1024L * 1024L
    }
}
