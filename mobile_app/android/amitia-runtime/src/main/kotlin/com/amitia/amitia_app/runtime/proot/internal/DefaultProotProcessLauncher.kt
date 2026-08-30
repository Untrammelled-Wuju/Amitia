package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.proot.ProotCommand
import com.amitia.amitia_app.runtime.proot.ProotErrorCode
import com.amitia.amitia_app.runtime.proot.ProotObserver
import com.amitia.amitia_app.runtime.proot.ProotProcessLauncher
import com.amitia.amitia_app.runtime.proot.ProotSession
import android.util.Log
import java.io.File

internal class DefaultProotProcessLauncher(
    private val sessionIdGenerator: SessionIdGenerator = UuidSessionIdGenerator(),
    private val nativeBackendPath: String = "",
) : ProotProcessLauncher {
    override fun launch(command: ProotCommand, observer: ProotObserver, generation: Long): ProotSession {
        val sessionId = sessionIdGenerator.generate()
        try {
            if (command.environment["AMITIA_RUNTIME_MODE"] == "android-native") {
                return launchNative(command, observer, generation, sessionId)
            }
            val fullCommand = ArrayList<String>(command.arguments.size + 1)
            fullCommand.add(command.binaryPath); fullCommand.addAll(command.arguments)
            Log.d("AmitiaRuntime", "proot-cmd: ${fullCommand.joinToString(" ")}")
            Log.d("AmitiaRuntime", "proot-env: ${command.environment.entries.joinToString(" ")}")

            val logFile = File(runRootOf(command.environment), "proot-launch.log")
            val shellCommand = ArrayList<String>()
            shellCommand.add("/system/bin/sh")
            shellCommand.add("-c")
            shellCommand.add("\"\$@\" >> \"\$0\" 2>&1")
            shellCommand.add(logFile.absolutePath)
            shellCommand.addAll(fullCommand)

            val pb = ProcessBuilder(shellCommand); pb.directory(File("/"))
            if (command.environment.isNotEmpty()) {
                pb.environment().remove("LD_LIBRARY_PATH")
                pb.environment().remove("LD_PRELOAD")
                pb.environment().putAll(command.environment)
            }
            val process = pb.start()
            Log.i("AmitiaRuntime", "proot-launched: alive=${runCatching { process.isAlive }.getOrElse { false }} log=${logFile.absolutePath}")
            val session = DefaultProotSession(
                sessionId = sessionId,
                process = process,
                observer = observer,
                generation = generation,
            )
            return session
        } catch (e: SecurityException) { throw ProotLaunchException(ProotErrorCode.PROCESS_START_FAILED, "security: ${e.message}", e) }
        catch (e: java.io.IOException) { throw ProotLaunchException(ProotErrorCode.PROCESS_START_FAILED, "io: ${e.message}", e) }
        catch (e: Exception) { throw ProotLaunchException(ProotErrorCode.PROCESS_START_FAILED, "failed: ${e.message}", e) }
    }

    private fun runRootOf(environment: Map<String, String>): File {
        val tmpDir = environment["PROOT_TMP_DIR"]
        if (!tmpDir.isNullOrBlank()) {
            val parent = File(tmpDir).parentFile
            if (parent != null) {
                parent.mkdirs()
                return parent
            }
        }
        return File("/")
    }

    private fun launchNative(
        command: ProotCommand,
        observer: ProotObserver,
        generation: Long,
        sessionId: String,
    ): ProotSession {
        val parsed = NativeCommand.parse(command.arguments)
        val executable = nativeBackendPath.takeIf { File(it).isFile } ?: parsed.mapGuestPath(parsed.program)
        val processBuilder = ProcessBuilder(listOf(executable) + parsed.programArguments)
        processBuilder.directory(File(parsed.mapGuestPath(parsed.workingDirectory)))
        val environment = processBuilder.environment()
        environment.remove("LD_LIBRARY_PATH")
        environment.remove("LD_PRELOAD")
        for ((key, value) in command.environment) {
            if (key == "PATH" || key == "AMITIA_RUNTIME_MODE") continue
            environment[key] = parsed.mapGuestPath(value)
        }
        return DefaultProotSession(
            sessionId = sessionId,
            process = processBuilder.start(),
            observer = observer,
            generation = generation,
        )
    }
}

private class NativeCommand(
    val workingDirectory: String,
    val mounts: List<Pair<String, String>>,
    val program: String,
    val programArguments: List<String>,
) {
    fun mapGuestPath(value: String): String {
        val mount = mounts.firstOrNull { (guest, _) -> value == guest || value.startsWith("$guest/") } ?: return value
        return mount.second + value.removePrefix(mount.first)
    }

    companion object {
        fun parse(arguments: List<String>): NativeCommand {
            var index = 0
            var workingDirectory = "/"
            val mounts = ArrayList<Pair<String, String>>()
            while (index < arguments.size) {
                when (arguments[index]) {
                    "-r" -> index += 2
                    "-w" -> {
                        workingDirectory = arguments.getOrNull(index + 1)
                            ?: throw IllegalArgumentException("missing native working directory")
                        index += 2
                    }
                    "-b" -> {
                        val binding = arguments.getOrNull(index + 1)
                            ?: throw IllegalArgumentException("missing native bind mount")
                        val delimiter = binding.lastIndexOf(':')
                        require(delimiter > 0 && delimiter < binding.lastIndex) { "invalid native bind mount" }
                        mounts.add(binding.substring(delimiter + 1) to binding.substring(0, delimiter))
                        index += 2
                    }
                    "--kill-on-exit", "-0" -> index += 1
                    else -> break
                }
            }
            val program = arguments.getOrNull(index) ?: throw IllegalArgumentException("missing native program")
            return NativeCommand(workingDirectory, mounts.sortedByDescending { it.first.length }, program, arguments.drop(index + 1))
        }
    }
}

internal class ProotLaunchException(val code: ProotErrorCode, override val message: String, cause: Throwable? = null) : RuntimeException(message, cause)
