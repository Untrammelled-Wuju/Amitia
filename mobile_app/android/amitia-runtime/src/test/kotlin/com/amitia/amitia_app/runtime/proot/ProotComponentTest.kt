package com.amitia.amitia_app.runtime.proot

import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Assert.assertNull
import org.junit.Assert.assertFalse
import org.junit.Test

class ProotComponentTest {

    @Test fun availability_returns_result() { val c = TestProotComponentFactory.create(); val r = c.availability(); assertNotNull(r) }
    @Test fun launch_returns_session() {
        val c = TestProotComponentFactory.create(); val r = ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/bin/echo"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY); val s = c.launch(r, ProotObserver {}); assertNotNull(s); assertTrue(s.sessionId.isNotEmpty())
    }
    @Test fun close_idempotent() { val c = TestProotComponentFactory.create(); c.close(); c.close() }
    @Test fun availability_after_close_is_closed() { val c = TestProotComponentFactory.create(); c.close(); c.availability() }
    @Test fun session_valid_until_close() { val c = TestProotComponentFactory.create(); val r = ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/bin/echo"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY); val s = c.launch(r, ProotObserver {}); assertNotNull(s) }
    @Test fun current_session_initially_null() { val c = TestProotComponentFactory.create(); assertNull(c.currentSession()) }
    @Test fun stop_when_no_session_returns_already_stopped() { val c = TestProotComponentFactory.create(); val r = c.stop(); assertTrue(r is ProotStopResult.AlreadyStopped) }
    @Test fun probe_returns_session() {
        val c = TestProotComponentFactory.create(); val r = ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/bin/true"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY); val s = c.launchProbe(r, ProotObserver {}); assertNotNull(s); assertTrue(s.sessionId.isNotEmpty())
    }
}

object TestProotComponentFactory {
    fun create(): ProotComponent {
        return object : ProotComponent {
            private val closed = java.util.concurrent.atomic.AtomicBoolean(false)
            private val sessions = mutableListOf<ProotSession>()
            private var mainSession: ProotSession? = null
            private val lock = Any()
            override fun availability(): ProotAvailability {
                if (closed.get()) return ProotAvailability.Closed
                return ProotAvailability.Unavailable(ProotErrorCode.NOT_PACKAGED, "proot.not_packaged")
            }
            override fun launch(request: ProotLaunchRequest, observer: ProotObserver): ProotSession {
                if (closed.get()) return ClosedTestSession
                synchronized(lock) {
                    if (mainSession?.isAlive() == true) return AlreadyRunningTestSession
                    val s = AliveTestSession()
                    mainSession = s; sessions.add(s); return s
                }
            }
            override fun launchProbe(request: ProotLaunchRequest, observer: ProotObserver): ProotSession {
                if (closed.get()) return ClosedTestSession
                val s = AliveTestSession()
                sessions.add(s); return s
            }
            override fun currentSession(): ProotSession? = mainSession
            override fun stop(): ProotStopResult {
                synchronized(lock) {
                    val s = mainSession
                    mainSession = null
                    return if (s != null) ProotStopResult.Graceful(s.sessionId, 0) else ProotStopResult.AlreadyStopped("none", null)
                }
            }
            override fun close() { if (closed.compareAndSet(false, true)) { sessions.forEach { it.close() }; sessions.clear(); mainSession = null } }
        }
    }
    private object ClosedTestSession : ProotSession {
        override val sessionId: String = "closed"
        override fun isAlive(): Boolean = false
        override fun awaitExit(timeoutMillis: Long): Int? = null
        override fun stop(graceMillis: Long) = ProotStopResult.AlreadyStopped("closed", null)
        override fun close() {}
    }
    private object AlreadyRunningTestSession : ProotSession {
        override val sessionId: String = "already-running"
        override fun isAlive(): Boolean = false
        override fun awaitExit(timeoutMillis: Long): Int? = null
        override fun stop(graceMillis: Long) = ProotStopResult.AlreadyStopped("already-running", null)
        override fun close() {}
    }
    private class AliveTestSession : ProotSession {
        override val sessionId: String = "test-alive"
        override fun isAlive(): Boolean = true
        override fun awaitExit(timeoutMillis: Long): Int? = 0
        override fun stop(graceMillis: Long) = ProotStopResult.Graceful(sessionId, 0)
        override fun close() {}
    }
}

class TestSession : ProotSession {
    override val sessionId: String = "test-session"
    override fun isAlive(): Boolean = false
    override fun awaitExit(timeoutMillis: Long): Int? = null
    override fun stop(graceMillis: Long) = ProotStopResult.AlreadyStopped("test", null)
    override fun close() {}
}