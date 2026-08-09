package com.amitia.amitia_app.runtime.recovery

import com.amitia.amitia_app.runtime.api.RuntimeError
import com.amitia.amitia_app.runtime.api.RuntimeErrorCode
import com.amitia.amitia_app.runtime.api.RuntimeState
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

internal class FakeInstalledRuntimeSource(private var result: InstalledRuntimeResult) : InstalledRuntimeSource {
    override fun current(): InstalledRuntimeResult = result
    fun setResult(r: InstalledRuntimeResult) { result = r }
}

class RuntimeCrashRecoveryPolicyTest {

    private fun createPolicy(
        installedSource: InstalledRuntimeSource = FakeInstalledRuntimeSource(InstalledRuntimeResult.Installed),
        maxAttempts: Int = 3,
        recoveryWindowMillis: Long = 5L * 60L * 1000L,
        stableReadyWindowMillis: Long = 60L * 1000L,
        clock: () -> Long = { 1000L },
    ): DefaultRuntimeCrashRecoveryPolicy {
        return DefaultRuntimeCrashRecoveryPolicy(
            installedRuntimeSource = installedSource,
            maxAttempts = maxAttempts,
            recoveryWindowMillis = recoveryWindowMillis,
            stableReadyWindowMillis = stableReadyWindowMillis,
            clock = clock,
        )
    }

    private fun recoverableError(): RuntimeError {
        return RuntimeError(code = RuntimeErrorCode.START_FAILED, message = "startup failed", recoverable = true)
    }

    private fun nonRecoverableError(): RuntimeError {
        return RuntimeError(code = RuntimeErrorCode.UNSUPPORTED_ABI, message = "unsupported abi", recoverable = false)
    }

    @Test
    fun requestedStop_doNotRecover() {
        val policy = createPolicy()
        val request = RuntimeRecoveryRequest(
            failedGeneration = 1L,
            currentState = RuntimeState.FAILED,
            error = recoverableError(),
            requestedStop = true,
        )
        val decision = policy.evaluate(request)
        assertTrue(decision is RuntimeRecoveryDecision.DoNotRecover)
    }

    @Test
    fun nonRecoverableError_doNotRecover() {
        val policy = createPolicy()
        val request = RuntimeRecoveryRequest(
            failedGeneration = 1L,
            currentState = RuntimeState.FAILED,
            error = nonRecoverableError(),
            requestedStop = false,
        )
        val decision = policy.evaluate(request)
        assertTrue(decision is RuntimeRecoveryDecision.DoNotRecover)
    }

    @Test
    fun noActiveRuntime_doNotRecover() {
        val source = FakeInstalledRuntimeSource(InstalledRuntimeResult.NoActiveRuntime)
        val policy = createPolicy(installedSource = source)
        val request = RuntimeRecoveryRequest(
            failedGeneration = 1L,
            currentState = RuntimeState.FAILED,
            error = recoverableError(),
            requestedStop = false,
        )
        val decision = policy.evaluate(request)
        assertTrue(decision is RuntimeRecoveryDecision.DoNotRecover)
    }

    @Test
    fun corruptedActiveRuntime_doNotRecover() {
        val source = FakeInstalledRuntimeSource(InstalledRuntimeResult.Corrupted("metadata invalid"))
        val policy = createPolicy(installedSource = source)
        val request = RuntimeRecoveryRequest(
            failedGeneration = 1L,
            currentState = RuntimeState.FAILED,
            error = recoverableError(),
            requestedStop = false,
        )
        val decision = policy.evaluate(request)
        assertTrue(decision is RuntimeRecoveryDecision.DoNotRecover)
    }

    @Test
    fun firstCrash_recoverAfter1s() {
        val policy = createPolicy()
        val request = RuntimeRecoveryRequest(
            failedGeneration = 1L,
            currentState = RuntimeState.FAILED,
            error = recoverableError(),
            requestedStop = false,
        )
        val decision = policy.evaluate(request)
        assertTrue(decision is RuntimeRecoveryDecision.RecoverAfter)
        assertEquals(1000L, (decision as RuntimeRecoveryDecision.RecoverAfter).delayMillis)
    }

    @Test
    fun secondCrash_recoverAfter2s() {
        val policy = createPolicy()
        policy.evaluate(RuntimeRecoveryRequest(1L, RuntimeState.FAILED, recoverableError(), false))
        val decision = policy.evaluate(RuntimeRecoveryRequest(2L, RuntimeState.FAILED, recoverableError(), false))
        assertTrue(decision is RuntimeRecoveryDecision.RecoverAfter)
        assertEquals(2000L, (decision as RuntimeRecoveryDecision.RecoverAfter).delayMillis)
    }

    @Test
    fun thirdCrash_recoverAfter4s() {
        val policy = createPolicy()
        policy.evaluate(RuntimeRecoveryRequest(1L, RuntimeState.FAILED, recoverableError(), false))
        policy.evaluate(RuntimeRecoveryRequest(2L, RuntimeState.FAILED, recoverableError(), false))
        val decision = policy.evaluate(RuntimeRecoveryRequest(3L, RuntimeState.FAILED, recoverableError(), false))
        assertTrue(decision is RuntimeRecoveryDecision.RecoverAfter)
        assertEquals(4000L, (decision as RuntimeRecoveryDecision.RecoverAfter).delayMillis)
    }

    @Test
    fun fourthCrash_exhausted() {
        val policy = createPolicy(maxAttempts = 3)
        policy.evaluate(RuntimeRecoveryRequest(1L, RuntimeState.FAILED, recoverableError(), false))
        policy.evaluate(RuntimeRecoveryRequest(2L, RuntimeState.FAILED, recoverableError(), false))
        policy.evaluate(RuntimeRecoveryRequest(3L, RuntimeState.FAILED, recoverableError(), false))
        val decision = policy.evaluate(RuntimeRecoveryRequest(4L, RuntimeState.FAILED, recoverableError(), false))
        assertTrue(decision is RuntimeRecoveryDecision.Exhausted)
        assertEquals(3, (decision as RuntimeRecoveryDecision.Exhausted).attempts)
    }

    @Test
    fun stableReady_resetsBudget() {
        val policy = createPolicy(stableReadyWindowMillis = 60000L, clock = { 100000L })
        policy.evaluate(RuntimeRecoveryRequest(1L, RuntimeState.FAILED, recoverableError(), false))
        policy.evaluate(RuntimeRecoveryRequest(2L, RuntimeState.FAILED, recoverableError(), false))
        policy.recordReady(3L)
        val lateClock = { 100000L + 61000L }
        val policy2 = createPolicy(stableReadyWindowMillis = 60000L, clock = lateClock)
        policy2.recordReady(3L)
        val decision = policy2.evaluate(RuntimeRecoveryRequest(4L, RuntimeState.FAILED, recoverableError(), false))
        assertTrue(decision is RuntimeRecoveryDecision.RecoverAfter)
        assertEquals(1000L, (decision as RuntimeRecoveryDecision.RecoverAfter).delayMillis)
    }

    @Test
    fun unstableReady_noReset() {
        val policy = createPolicy(stableReadyWindowMillis = 60000L)
        policy.recordReady(1L)
        val decision = policy.evaluate(RuntimeRecoveryRequest(2L, RuntimeState.FAILED, recoverableError(), false))
        assertTrue(decision is RuntimeRecoveryDecision.RecoverAfter)
        assertEquals(1000L, (decision as RuntimeRecoveryDecision.RecoverAfter).delayMillis)
    }

    @Test
    fun cancelPending_resetsBudget() {
        val policy = createPolicy()
        policy.evaluate(RuntimeRecoveryRequest(1L, RuntimeState.FAILED, recoverableError(), false))
        policy.evaluate(RuntimeRecoveryRequest(2L, RuntimeState.FAILED, recoverableError(), false))
        policy.cancelPending()
        val decision = policy.evaluate(RuntimeRecoveryRequest(3L, RuntimeState.FAILED, recoverableError(), false))
        assertTrue(decision is RuntimeRecoveryDecision.RecoverAfter)
        assertEquals(1000L, (decision as RuntimeRecoveryDecision.RecoverAfter).delayMillis)
    }

    @Test
    fun startupCancelled_nonRecoverable() {
        val policy = createPolicy()
        val request = RuntimeRecoveryRequest(
            failedGeneration = 1L,
            currentState = RuntimeState.FAILED,
            error = RuntimeError(code = RuntimeErrorCode.STARTUP_CANCELLED, message = "cancelled", recoverable = true),
            requestedStop = false,
        )
        val decision = policy.evaluate(request)
        assertTrue(decision is RuntimeRecoveryDecision.DoNotRecover)
    }

    @Test
    fun startupGenerationStale_nonRecoverable() {
        val policy = createPolicy()
        val request = RuntimeRecoveryRequest(
            failedGeneration = 1L,
            currentState = RuntimeState.FAILED,
            error = RuntimeError(code = RuntimeErrorCode.STARTUP_GENERATION_STALE, message = "stale", recoverable = true),
            requestedStop = false,
        )
        val decision = policy.evaluate(request)
        assertTrue(decision is RuntimeRecoveryDecision.DoNotRecover)
    }
}
