package com.amitia.amitia_app.nativeprovider.shizuku

import android.content.ComponentName
import android.os.IBinder
import android.os.Parcel
import android.os.RemoteException
import org.json.JSONArray
import org.json.JSONObject
import rikka.shizuku.Shizuku
import java.io.BufferedReader
import java.io.File
import java.io.InputStreamReader
import java.util.concurrent.Callable
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean

class ShizukuCommandService : IPrivilegedCommandService.Stub() {

    private val ioExecutor = Executors.newFixedThreadPool(2)
    private val destroyed = AtomicBoolean(false)
    private val processManager = ShizukuProcessManager()

    override fun asBinder(): IBinder = this as IBinder

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
                request.env,
                request.workDir,
                request.timeoutMs,
                request.maxOutputBytes
            )
            serializeResult(result)
        } catch (e: Exception) {
            JSONObject()
                .put(
                    "error",
                    JSONObject()
                        .put("code", "EXECUTION_ERROR")
                        .put("message", e.message ?: "unknown"),
                )
                .toString()
        }
    }

    override fun destroy() {
        if (destroyed.compareAndSet(false, true)) {
            try {
                processManager.destroyAll()
                ioExecutor.shutdownNow()
                ioExecutor.awaitTermination(250, TimeUnit.MILLISECONDS)
            } catch (_: Exception) {
            } finally {
                System.exit(0)
            }
        }
    }

    override fun startProcess(requestJson: String): String = processManager.start(requestJson)

    override fun writeProcess(requestJson: String): String = processManager.write(requestJson)

    override fun readProcess(requestJson: String): String = processManager.read(requestJson)

    override fun waitProcess(requestJson: String): String = processManager.wait(requestJson)

    override fun killProcess(requestJson: String): String = processManager.kill(requestJson)

    private fun parseRequest(json: String): ShizukuCommandRequest {
        val objectValue = JSONObject(json)
        val argsArray = objectValue.optJSONArray("args") ?: JSONArray()
        val args = (0 until argsArray.length()).map { index ->
            argsArray.opt(index).toString()
        }
        val envObject = objectValue.optJSONObject("env") ?: JSONObject()
        val env = mutableMapOf<String, String>()
        val envKeys = envObject.keys()
        while (envKeys.hasNext()) {
            val key = envKeys.next()
            env[key] = envObject.optString(key)
        }
        return ShizukuCommandRequest(
            executable = objectValue.optString("executable"),
            args = args,
            stdin = if (objectValue.isNull("stdin")) null else objectValue.optString("stdin"),
            env = env,
            workDir = if (objectValue.isNull("workDir")) null else objectValue.optString("workDir"),
            timeoutMs = objectValue.optLong("timeoutMs", 30000L),
            maxOutputBytes = objectValue.optLong("maxOutputBytes", 1048576L),
        )
    }

    private fun serializeResult(result: ShizukuCommandResult): String {
        return JSONObject()
            .put("exitCode", result.exitCode)
            .put("exitCodeAvailable", result.exitCodeAvailable)
            .put("stdout", result.stdout)
            .put("stderr", result.stderr)
            .put("durationMs", result.durationMs)
            .put("timedOut", result.timedOut)
            .toString()
    }

    private fun executeCommandInternal(
        executable: String,
        args: List<*>,
        stdin: String?,
        env: Map<String, String>,
        workDir: String?,
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
            if (env.isNotEmpty()) {
                pb.environment().putAll(env)
            }
            if (!workDir.isNullOrBlank()) {
                pb.directory(File(workDir))
            }
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

            val stdoutFuture = ioExecutor.submit(Callable<String> {
                readBounded(process.inputStream, maxOutputBytes)
            })

            val stderrFuture = ioExecutor.submit(Callable<String> {
                readBounded(process.errorStream, maxOutputBytes)
            })

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
        fun createArgs(): Shizuku.UserServiceArgs {
            return Shizuku.UserServiceArgs(
                ComponentName(
                    "com.amitia.amitia_app",
                    ShizukuCommandService::class.java.name,
                )
            )
                .daemon(false)
                .processNameSuffix("shizuku_command")
                .debuggable(false)
                .version(14)
        }
    }
}
