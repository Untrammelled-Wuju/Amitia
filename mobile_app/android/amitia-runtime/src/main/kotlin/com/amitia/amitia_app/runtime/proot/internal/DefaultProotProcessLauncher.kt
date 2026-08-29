package com.amitia.amitia_app.runtime.proot.internal

import android.util.Log
import com.amitia.amitia_app.runtime.proot.ProotCommand
import com.amitia.amitia_app.runtime.proot.ProotErrorCode
import com.amitia.amitia_app.runtime.proot.ProotObserver
import com.amitia.amitia_app.runtime.proot.ProotProcessLauncher
import com.amitia.amitia_app.runtime.proot.ProotSession
import java.io.File

internal class DefaultProotProcessLauncher(private val sessionIdGenerator: SessionIdGenerator = UuidSessionIdGenerator()) : ProotProcessLauncher {
    override fun launch(command: ProotCommand, observer: ProotObserver, generation: Long): ProotSession {
        val sessionId = sessionIdGenerator.generate()
        try {
            val fullCommand = ArrayList<String>(command.arguments.size + 1)
            fullCommand.add(command.binaryPath); fullCommand.addAll(command.arguments)
            Log.e("AmitiaRuntime", fullCommand.joinToString(separator = "\n"))
            val shellCommand = ArrayList<String>(fullCommand.size + 4)
            shellCommand.add("/system/bin/sh")
            shellCommand.add("-c")
            shellCommand.add("exec \"\$@\"")
            shellCommand.add("amitia-proot")
            shellCommand.addAll(fullCommand)
            val pb = ProcessBuilder(shellCommand); pb.directory(File("/"))
            if (command.environment.isNotEmpty()) {
                pb.environment().remove("LD_LIBRARY_PATH")
                pb.environment().remove("LD_PRELOAD")
                pb.environment().putAll(command.environment)
            }
            val process = pb.start()
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
}

internal class ProotLaunchException(val code: ProotErrorCode, override val message: String, cause: Throwable? = null) : RuntimeException(message, cause)
