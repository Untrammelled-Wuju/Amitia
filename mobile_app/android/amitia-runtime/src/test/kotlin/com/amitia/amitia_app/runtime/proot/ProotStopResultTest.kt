package com.amitia.amitia_app.runtime.proot

import org.junit.Assert.assertEquals
import org.junit.Test

class ProotStopResultTest {
    @Test fun graceful_sets_fields() { val r = ProotStopResult.Graceful("s1", 0); assertEquals("s1", r.sessionId); assertEquals(0, r.exitCode) }
    @Test fun forced_with_code() { val r = ProotStopResult.Forced("s2", -1); assertEquals("s2", r.sessionId); assertEquals(-1, r.exitCode) }
    @Test fun forced_without_code() { val r = ProotStopResult.Forced("s3", null); assertEquals("s3", r.sessionId); assertEquals(null, r.exitCode) }
    @Test fun already_stopped() { val r = ProotStopResult.AlreadyStopped("s4", 42); assertEquals("s4", r.sessionId); assertEquals(42, r.exitCode) }
    @Test fun failed() { val r = ProotStopResult.Failed("s5", ProotErrorCode.PROCESS_STOP_FAILED, "error"); assertEquals("s5", r.sessionId); assertEquals(ProotErrorCode.PROCESS_STOP_FAILED, r.errorCode) }
}