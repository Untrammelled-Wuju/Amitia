package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.abi.RuntimeAbiGate
import com.amitia.amitia_app.runtime.abi.RuntimeAbiStatus
import com.amitia.amitia_app.runtime.proot.ProotAvailability
import com.amitia.amitia_app.runtime.proot.ProotBinaryLocator
import com.amitia.amitia_app.runtime.proot.ProotCommandBuilder
import com.amitia.amitia_app.runtime.proot.ProotComponent
import com.amitia.amitia_app.runtime.proot.ProotErrorCode
import com.amitia.amitia_app.runtime.proot.ProotLaunchRequest
import com.amitia.amitia_app.runtime.proot.ProotObserver
import com.amitia.amitia_app.runtime.proot.ProotProcessLauncher
import com.amitia.amitia_app.runtime.proot.ProotSession
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicReference

internal class AndroidProotComponent(
    private val binaryLocator: ProotBinaryLocator,
    private val artifactVerifier: DefaultProotArtifactVerifier,
    private val commandBuilder: ProotCommandBuilder,
    private val processLauncher: ProotProcessLauncher,
    private val abiGate: RuntimeAbiGate? = null
) : ProotComponent {

    private val closed = AtomicBoolean(false)
    private val cachedAvailability = AtomicReference<ProotAvailability?>(null)
    private val activeSessions = ConcurrentHashMap<String, ProotSession>()

    override fun availability(): ProotAvailability {
        if (closed.get()) return ProotAvailability.Closed
        val abiStatus = abiGate?.evaluate()
        if (abiStatus != null && abiStatus !is RuntimeAbiStatus.Supported) {
            return ProotAvailability.Unavailable(ProotErrorCode.UNSUPPORTED_ABI, "proot.abi.unsupported")
        }
        val cached = cachedAvailability.get()
        if (cached != null) return cached
        val result = artifactVerifier.verify()
        if (result is ProotAvailability.Available) cachedAvailability.compareAndSet(null, result)
        return result
    }

    override fun launch(request: ProotLaunchRequest, observer: ProotObserver): ProotSession {
        if (closed.get()) return ClosedSession
        val abiStatus = abiGate?.evaluate()
        if (abiStatus != null && abiStatus !is RuntimeAbiStatus.Supported) {
            return ClosedSession
        }
        val avail = availability()
        if (avail !is ProotAvailability.Available) return ClosedSession
        val command = commandBuilder.build(avail.absoluteBinaryPath, request)
        val session = processLauncher.launch(command, observer)
        activeSessions[session.sessionId] = session
        return session
    }

    override fun close() {
        if (closed.compareAndSet(false, true)) {
            for (session in activeSessions.values) { try { session.close() } catch (_: Throwable) {} }
            activeSessions.clear()
        }
    }

    private object ClosedSession : ProotSession {
        override val sessionId: String = "closed"
        override fun isAlive(): Boolean = false
        override fun awaitExit(timeoutMillis: Long): Int? = null
        override fun stop(graceMillis: Long) = com.amitia.amitia_app.runtime.proot.ProotStopResult.AlreadyStopped("closed", null)
        override fun close() {}
    }
}