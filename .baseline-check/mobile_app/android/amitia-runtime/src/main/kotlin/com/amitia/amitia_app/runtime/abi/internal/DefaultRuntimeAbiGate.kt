package com.amitia.amitia_app.runtime.abi.internal

import com.amitia.amitia_app.runtime.abi.AndroidAbiProvider
import com.amitia.amitia_app.runtime.abi.RuntimeAbiError
import com.amitia.amitia_app.runtime.abi.RuntimeAbiErrorCode
import com.amitia.amitia_app.runtime.abi.RuntimeAbiGate
import com.amitia.amitia_app.runtime.abi.RuntimeAbiPolicy
import com.amitia.amitia_app.runtime.abi.RuntimeAbiSnapshot
import com.amitia.amitia_app.runtime.abi.RuntimeAbiStatus
import com.amitia.amitia_app.runtime.abi.UnsupportedReason
import java.util.concurrent.atomic.AtomicReference

internal class DefaultRuntimeAbiGate(
    private val policy: RuntimeAbiPolicy = RuntimeAbiPolicy.amitiaAndroid(),
    private val provider: AndroidAbiProvider
) : RuntimeAbiGate {

    private val cachedResult = AtomicReference<CacheEntry?>(null)

    override fun evaluate(): RuntimeAbiStatus {
        val cached = cachedResult.get()
        if (cached != null && cached.status !is RuntimeAbiStatus.DetectionFailed) {
            return cached.status
        }

        val snapshot = provider.snapshot()
        val result = evaluateSnapshot(snapshot)

        if (result !is RuntimeAbiStatus.DetectionFailed) {
            cachedResult.compareAndSet(cached, CacheEntry(snapshot, result))
        }

        return result
    }

    override fun isSupported(): Boolean {
        return evaluate() is RuntimeAbiStatus.Supported
    }

    private fun evaluateSnapshot(snapshot: RuntimeAbiSnapshot): RuntimeAbiStatus {
        val supported = snapshot.supportedAbis
        if (supported.isEmpty()) {
            return RuntimeAbiStatus.Unsupported(UnsupportedReason.SUPPORTED_ABIS_EMPTY, snapshot)
        }

        if (!RuntimeAbiValidators.containsArm64(supported)) {
            return RuntimeAbiStatus.Unsupported(UnsupportedReason.ARM64_ABI_MISSING, snapshot)
        }

        val supported64 = snapshot.supported64BitAbis
        if (supported64.isEmpty() || !RuntimeAbiValidators.containsArm64(supported64)) {
            return RuntimeAbiStatus.Unsupported(UnsupportedReason.ARM64_64_BIT_ABI_MISSING, snapshot)
        }

        val is64Bit = snapshot.processIs64Bit
        if (is64Bit != null && !is64Bit) {
            return RuntimeAbiStatus.Unsupported(UnsupportedReason.PROCESS_IS_32_BIT, snapshot)
        }

        return RuntimeAbiStatus.Supported(
            abi = policy.required64BitAbi,
            processIs64Bit = is64Bit,
            snapshot = snapshot
        )
    }

    private data class CacheEntry(
        val snapshot: RuntimeAbiSnapshot,
        val status: RuntimeAbiStatus
    )
}