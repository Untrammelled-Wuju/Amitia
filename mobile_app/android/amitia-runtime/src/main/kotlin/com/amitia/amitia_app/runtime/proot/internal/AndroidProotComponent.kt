package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.abi.RuntimeAbiGate
import com.amitia.amitia_app.runtime.abi.RuntimeAbiStatus
import com.amitia.amitia_app.runtime.proot.ProotAvailability
import com.amitia.amitia_app.runtime.proot.ProotBinaryLocator
import com.amitia.amitia_app.runtime.proot.ProotCommandBuilder
import com.amitia.amitia_app.runtime.proot.ProotComponent
import com.amitia.amitia_app.runtime.proot.ProotErrorCode
import com.amitia.amitia_app.runtime.proot.ProotLaunchRequest
import com.amitia.amitia_app.runtime.proot.ProotLaunchSpec
import com.amitia.amitia_app.runtime.proot.ProotObserver
import com.amitia.amitia_app.runtime.proot.ProotProcessLauncher
import com.amitia.amitia_app.runtime.proot.ProotSession
import com.amitia.amitia_app.runtime.proot.ProotStopResult
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicReference

internal class AndroidProotComponent(
    private val binaryLocator: ProotBinaryLocator,
    private val artifactVerifier: ProotArtifactVerifier,
    private val commandBuilder: ProotCommandBuilder,
    private val processLauncher: ProotProcessLauncher,
    private val abiGate: RuntimeAbiGate? = null
) : ProotComponent {

    private val closed = AtomicBoolean(false)
    private val cachedAvailability = AtomicReference<ProotAvailability?>(null)
    private val activeSessions = ConcurrentHashMap<String, ProotSession>()
    private val mainSession = AtomicReference<ProotSession?>(null)
    private val lock = Any()

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

    override fun launch(request: ProotLaunchRequest, observer: ProotObserver, generation: Long): ProotSession {
        if (closed.get()) return ClosedSession
        val abiStatus = abiGate?.evaluate()
        if (abiStatus != null && abiStatus !is RuntimeAbiStatus.Supported) {
            return ClosedSession
        }
        val avail = availability()
        if (avail !is ProotAvailability.Available) return ClosedSession
        synchronized(lock) {
            cleanupDeadSessions()
            val existing = mainSession.get()
            if (existing != null && existing.isAlive()) {
                return ClosedSession
            }
            val spec = ProotLaunchSpec.from(request, avail.absoluteBinaryPath)
            val command = commandBuilder.build(spec)
            val session = processLauncher.launch(command, observer, generation)
            mainSession.set(session)
            activeSessions[session.sessionId] = session
            return session
        }
    }

    override fun launchProbe(request: ProotLaunchRequest, observer: ProotObserver, generation: Long): ProotSession {
        if (closed.get()) return ClosedSession
        val abiStatus = abiGate?.evaluate()
        if (abiStatus != null && abiStatus !is RuntimeAbiStatus.Supported) {
            return ClosedSession
        }
        val avail = availability()
        if (avail !is ProotAvailability.Available) return ClosedSession
        synchronized(lock) {
            cleanupDeadSessions()
            val existing = mainSession.get()
            if (existing != null && existing.isAlive()) {
                return ClosedSession
            }
            val spec = ProotLaunchSpec.from(request, avail.absoluteBinaryPath)
            val command = commandBuilder.build(spec)
            val session = processLauncher.launch(command, observer, generation)
            mainSession.set(session)
            activeSessions[session.sessionId] = session
            return session
        }
    }

    override fun currentSession(): ProotSession? {
        synchronized(lock) {
            cleanupDeadSessions()
            return mainSession.get()
        }
    }

    override fun stop(): ProotStopResult {
        synchronized(lock) {
            cleanupDeadSessions()
            val session = mainSession.getAndSet(null) ?: return ProotStopResult.AlreadyStopped("none", null)
            session.requestStop()
            val result = session.stop(10_000L)
            activeSessions.remove(session.sessionId)
            return result
        }
    }

    override fun close() {
        if (closed.compareAndSet(false, true)) {
            synchronized(lock) {
                mainSession.set(null)
                for (session in activeSessions.values) { try { session.close() } catch (_: Throwable) {} }
                activeSessions.clear()
            }
        }
    }

    private fun cleanupDeadSessions() {
        val dead = activeSessions.entries.filter { !it.value.isAlive() }.map { it.key }
        dead.forEach { activeSessions.remove(it) }
        val main = mainSession.get()
        if (main != null && !main.isAlive()) mainSession.compareAndSet(main, null)
    }

    private object ClosedSession : ProotSession {
        override val sessionId: String = "closed"
        override fun isAlive(): Boolean = false
        override fun awaitExit(timeoutMillis: Long): Int? = null
        override fun stop(graceMillis: Long) = ProotStopResult.AlreadyStopped("closed", null)
        override fun close() {}
        override fun requestStop() {}
        override val exit: com.amitia.amitia_app.runtime.proot.ProotExit? = null
    }
}
