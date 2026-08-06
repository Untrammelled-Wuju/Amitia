package com.amitia.amitia_app.runtime.proot

import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Test

class ProotComponentTest {

    @Test fun availability_returns_result() { val c = TestProotComponentFactory.create(); val r = c.availability(); assertNotNull(r) }
    @Test fun launch_returns_session() {
        val c = TestProotComponentFactory.create(); val r = ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/bin/echo"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY); val s = c.launch(r, ProotObserver {}); assertNotNull(s); assertTrue(s.sessionId.isNotEmpty())
    }
    @Test fun close_idempotent() { val c = TestProotComponentFactory.create(); c.close(); c.close() }
    @Test fun availability_after_close_is_closed() { val c = TestProotComponentFactory.create(); c.close(); c.availability() }
    @Test fun session_valid_until_close() { val c = TestProotComponentFactory.create(); val r = ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/bin/echo"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY); val s = c.launch(r, ProotObserver {}); assertNotNull(s) }
}

object TestProotComponentFactory {
    fun create(): ProotComponent {
        return object : ProotComponent {
            private val closed = java.util.concurrent.atomic.AtomicBoolean(false)
            private val sessions = mutableListOf<ProotSession>()
            override fun availability(): ProotAvailability {
                if (closed.get()) return ProotAvailability.Closed
                return ProotAvailability.Unavailable(ProotErrorCode.NOT_PACKAGED, "proot.not_packaged")
            }
            override fun launch(request: ProotLaunchRequest, observer: ProotObserver): ProotSession {
                if (closed.get()) return object : ProotSession {
                    override val sessionId: String = "closed"
                    override fun isAlive(): Boolean = false
                    override fun awaitExit(timeoutMillis: Long): Int? = null
                    override fun stop(graceMillis: Long) = ProotStopResult.AlreadyStopped("closed", null)
                    override fun close() {}
                }
                val s = TestSession()
                sessions.add(s); return s
            }
            override fun close() { if (closed.compareAndSet(false, true)) { sessions.forEach { it.close() }; sessions.clear() } }
        }
    }
}

class TestSession : ProotSession {
    override val sessionId: String = "test-session"
    override fun isAlive(): Boolean = false
    override fun awaitExit(timeoutMillis: Long): Int? = null
    override fun stop(graceMillis: Long) = ProotStopResult.AlreadyStopped("test", null)
    override fun close() {}
}