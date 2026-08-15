package com.amitia.amitia_app.nativeprovider.shizuku

import android.content.ComponentName
import android.os.IBinder
import android.os.Parcel
import android.os.RemoteException
import rikka.shizuku.Shizuku
import rikka.shizuku.ShizukuUserServiceArgs
import java.io.BufferedReader
import java.io.InputStreamReader
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean

class ShizukuCommandService : IPrivilegedCommandService.Stub() {

    private val ioExecutor = Executors.newFixedThreadPool(2)
    private val destroyed = AtomicBoolean(false)

    override fun onTransact(code: Int, data: Parcel, reply: Parcel?, flags: Int): Boolean {
        if (destroyed.get()) {
            return false
        }
        return try {
            super.onTransact(code, data, reply, flags)
        } catch (e: Exception) {
            false
        }
    }

    override fun executeCommand(requestJson: String): String {
        if (destroyed.get()) {
            return """{"error":{"code":"SERVICE_DESTROYED","message":"service has been destroyed"}}"""
        }

        return try {
            val request = parseRequest(requestJson)
            val result = executeCommandInternal(
                request.executable,
                request.args,
                request.stdin,
                request.timeoutMs,
                request.maxOutputBytes
            )
            serializeResult(result)
        } catch (e: Exception) {
            """{"error":{"code":"EXECUTION_ERROR","message":"${e.message?.replace("\"", "\\\"") ?: "unknown"}"}}"""
        }
    }

    override fun destroyService() {
        if (destroyed.compareAndSet(false, true)) {
            ioExecutor.shutdownNow()
        }
    }

    private fun parseRequest(json: String): ShizukuCommandRequest {
        val map = mutableMapOf<String, Any?>()
        val cleaned = json.trim().removePrefix("{").removeSuffix("}").trim()
        if (cleaned.isNotEmpty()) {
            var i = 0
            while (i < cleaned.length) {
                while (i < cleaned.length && cleaned[i] in " \t\r\n") i++
                if (i >= cleaned.length || cleaned[i] != '"') break
                i++
                val keyStart = i
                while (i < cleaned.length && cleaned[i] != '"') i++
                val key = cleaned.substring(keyStart, i)
                i++
                while (i < cleaned.length && cleaned[i] != ':') i++
                i++
                while (i < cleaned.length && cleaned[i] in " \t\r\n") i++
                if (i >= cleaned.length) break
                when {
                    cleaned[i] == '"' -> {
                        i++
                        val sb = StringBuilder()
                        while (i < cleaned.length && cleaned[i] != '"') {
                            if (cleaned[i] == '\\' && i + 1 < cleaned.length) {
                                sb.append(cleaned[i + 1])
                                i += 2
                            } else {
                                sb.append(cleaned[i])
                                i++
                            }
                        }
                        map[key] = sb.toString()
                        i++
                    }
                    cleaned[i] == '[' -> {
                        val list = mutableListOf<String>()
                        i++
                        while (i < cleaned.length && cleaned[i] != ']') {
                            while (i < cleaned.length && cleaned[i] in " \t\r\n,") i++
                            if (i < cleaned.length && cleaned[i] == '"') {
                                i++
                                val sb = StringBuilder()
                                while (i < cleaned.length && cleaned[i] != '"') {
                                    if (cleaned[i] == '\\' && i + 1 < cleaned.length) {
                                        sb.append(cleaned[i + 1])
                                        i += 2
                                    } else {
                                        sb.append(cleaned[i])
                                        i++
                                    }
                                }
                                list.add(sb.toString())
                                i++
                            } else {
                                break
                            }
                        }
                        map[key] = list
                        i++
                    }
                    cleaned[i].isDigit() || cleaned[i] == '-' -> {
                        val numStart = i
                        while (i < cleaned.length && cleaned[i] in "-0123456789.") i++
                        val numStr = cleaned.substring(numStart, i)
                        map[key] = numStr.toLongOrNull() ?: numStr.toDoubleOrNull() ?: 0L
                    }
                    cleaned.startsWith("true", i) -> {
                        map[key] = true
                        i += 4
                    }
                    cleaned.startsWith("false", i) -> {
                        map[key] = false
                        i += 5
                    }
                    else -> break
                }
                while (i < cleaned.length && cleaned[i] in " \t\r\n,") i++
            }
        }

        return ShizukuCommandRequest(
            executable = map["executable"] as? String ?: "",
            args = (map["args"] as? List<*>)?.map { it.toString() } ?: emptyList(),
            stdin = map["stdin"] as? String,
            timeoutMs = (map["timeoutMs"] as? Number)?.toLong() ?: 30000L,
            maxOutputBytes = (map["maxOutputBytes"] as? Number)?.toLong() ?: 1048576L,
        )
    }

    private fun serializeResult(result: ShizukuCommandResult): String {
        val stdoutEscaped = result.stdout.replace("\\", "\\\\").replace("\"", "\\\"").replace("\n", "\\n").replace("\r", "\\r")
        val stderrEscaped = result.stderr.replace("\\", "\\\\").replace("\"", "\\\"").replace("\n", "\\n").replace("\r", "\\r")
        return """{"exitCode":${result.exitCode},"exitCodeAvailable":${result.exitCodeAvailable},"stdout":"$stdoutEscaped","stderr":"$stderrEscaped","durationMs":${result.durationMs},"timedOut":${result.timedOut}}"""
    }

    private fun executeCommandInternal(
        executable: String,
        args: List<*>,
        stdin: String?,
        timeoutMs: Long,
        maxOutputBytes: Long,
    ): ShizukuCommandResult {
        val startTime = System.currentTimeMillis()
        var process: Process? = null

        return try {
            val command = mutableListOf(executable)
            command.addAll(args.map { it.toString() })

            val pb = ProcessBuilder(command)
            pb.redirectErrorStream(false)
            process = pb.start()

            if (!stdin.isNullOrEmpty()) {
                process.outputStream.use { os ->
                    os.write(stdin.toByteArray(Charsets.UTF_8))
                    os.flush()
                }
            } else {
                process.outputStream.close()
            }

            val timedOut = AtomicBoolean(false)

            val stdoutFuture = ioExecutor.submit {
                readBounded(process.inputStream, maxOutputBytes)
            }

            val stderrFuture = ioExecutor.submit {
                readBounded(process.errorStream, maxOutputBytes)
            }

            val finished = process.waitFor(timeoutMs, TimeUnit.MILLISECONDS)

            if (!finished) {
                timedOut.set(true)
                process.destroy()
                if (!process.waitFor(200, TimeUnit.MILLISECONDS)) {
                    process.destroyForcibly()
                }
            }

            val stdout = stdoutFuture.get(500, TimeUnit.MILLISECONDS)
            val stderr = stderrFuture.get(500, TimeUnit.MILLISECONDS)

            val duration = System.currentTimeMillis() - startTime

            ShizukuCommandResult(
                exitCode = if (finished && !timedOut.get()) process.exitValue() else -1,
                exitCodeAvailable = finished && !timedOut.get(),
                stdout = stdout,
                stderr = stderr,
                durationMs = duration,
                timedOut = timedOut.get(),
            )
        } catch (e: Exception) {
            val duration = System.currentTimeMillis() - startTime
            try {
                process?.destroyForcibly()
            } catch (_: Exception) {}

            ShizukuCommandResult(
                exitCode = -1,
                exitCodeAvailable = false,
                stdout = "",
                stderr = e.message ?: "execution error",
                durationMs = duration,
                timedOut = false,
            )
        }
    }

    private fun readBounded(stream: java.io.InputStream, maxBytes: Long): String {
        val sb = StringBuilder()
        val reader = BufferedReader(InputStreamReader(stream, Charsets.UTF_8))
        var totalBytes = 0L
        try {
            var line: String?
            while (reader.readLine().also { line = it } != null) {
                val lineBytes = (line?.toByteArray(Charsets.UTF_8)?.size ?: 0) + 1
                if (totalBytes + lineBytes > maxBytes) break
                totalBytes += lineBytes
                sb.append(line).append("\n")
                if (Thread.currentThread().isInterrupted) break
            }
        } catch (_: Exception) {}
        return sb.toString()
    }

    companion object {
        const val TRANSACTION_executeCommand: Int = IBinder.FIRST_CALL_TRANSACTION
        const val TRANSACTION_destroyService: Int = IBinder.FIRST_CALL_TRANSACTION + 1

        fun createArgs(): ShizukuUserServiceArgs<ShizukuCommandService> {
            return ShizukuUserServiceArgs(
                ComponentName(
                    "com.amitia.amitia_app",
                    ShizukuCommandService::class.java.name,
                )
            )
                .daemon(false)
                .processNameSuffix("shizuku_command")
                .debuggable(false)
        }
    }
}
