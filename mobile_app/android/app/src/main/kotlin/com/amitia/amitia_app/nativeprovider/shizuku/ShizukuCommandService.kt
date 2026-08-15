package com.amitia.amitia_app.nativeprovider.shizuku

import android.content.ComponentName
import rikka.shizuku.Shizuku
import rikka.shizuku.ShizukuUserServiceArgs
import java.io.BufferedReader
import java.io.InputStreamReader
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean

class ShizukuCommandService : Shizuku.UserService() {

    private val ioExecutor = Executors.newFixedThreadPool(2)

    override fun onCreate() {
        super.onCreate()
        ShizukuCommandServiceHolder.onServiceCreated(this)
    }

    override fun onDestroy() {
        super.onDestroy()
        ShizukuCommandServiceHolder.onServiceDestroyed(this)
        ioExecutor.shutdownNow()
    }

    fun executeCommand(
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
                    os.write(stdin.toByteArray())
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
        val reader = BufferedReader(InputStreamReader(stream))
        var totalBytes = 0L
        try {
            var line: String?
            while (reader.readLine().also { line = it } != null) {
                totalBytes += (line?.length ?: 0) + 1
                if (totalBytes > maxBytes) break
                sb.append(line).append("\n")
                if (Thread.currentThread().isInterrupted) break
            }
        } catch (_: Exception) {}
        return sb.toString()
    }

    companion object {
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
