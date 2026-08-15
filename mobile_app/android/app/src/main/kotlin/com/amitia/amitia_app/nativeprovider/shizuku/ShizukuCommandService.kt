package com.amitia.amitia_app.nativeprovider.shizuku

import android.content.ComponentName
import rikka.shizuku.Shizuku
import rikka.shizuku.ShizukuUserServiceArgs
import java.io.BufferedReader
import java.io.InputStreamReader
import java.util.concurrent.TimeUnit

class ShizukuCommandService : Shizuku.UserService() {

    override fun onCreate() {
        super.onCreate()
        ShizukuCommandServiceHolder.service = this
    }

    override fun onDestroy() {
        super.onDestroy()
        ShizukuCommandServiceHolder.service = null
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

            val stdoutBuilder = StringBuilder()
            val stderrBuilder = StringBuilder()

            val stdoutReader = BufferedReader(InputStreamReader(process.inputStream))
            val stderrReader = BufferedReader(InputStreamReader(process.errorStream))

            var totalBytes = 0L
            var line: String?

            while (totalBytes < maxOutputBytes) {
                val stdoutAvailable = process.inputStream.available() > 0
                val stderrAvailable = process.errorStream.available() > 0

                if (!stdoutAvailable && !stderrAvailable) {
                    val exited = try {
                        process.exitValue()
                        true
                    } catch (e: IllegalThreadStateException) {
                        false
                    }
                    if (exited) break
                    Thread.sleep(10)
                    continue
                }

                if (stdoutAvailable) {
                    line = stdoutReader.readLine()
                    if (line != null) {
                        totalBytes += line.length
                        if (totalBytes <= maxOutputBytes) {
                            stdoutBuilder.append(line).append("\n")
                        }
                    }
                }

                if (stderrAvailable) {
                    line = stderrReader.readLine()
                    if (line != null) {
                        totalBytes += line.length
                        if (totalBytes <= maxOutputBytes) {
                            stderrBuilder.append(line).append("\n")
                        }
                    }
                }
            }

            val finished = process.waitFor(timeoutMs, TimeUnit.MILLISECONDS)
            val duration = System.currentTimeMillis() - startTime

            if (!finished) {
                process.destroyForcibly()
                ShizukuCommandResult(
                    exitCode = -1,
                    exitCodeAvailable = false,
                    stdout = stdoutBuilder.toString(),
                    stderr = stderrBuilder.toString(),
                    durationMs = duration,
                    timedOut = true,
                )
            } else {
                ShizukuCommandResult(
                    exitCode = process.exitValue(),
                    exitCodeAvailable = true,
                    stdout = stdoutBuilder.toString(),
                    stderr = stderrBuilder.toString(),
                    durationMs = duration,
                    timedOut = false,
                )
            }
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
