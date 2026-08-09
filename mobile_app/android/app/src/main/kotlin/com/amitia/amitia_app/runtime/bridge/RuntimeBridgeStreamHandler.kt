package com.amitia.amitia_app.runtime.bridge

import com.amitia.amitia_app.runtime.api.RuntimeController
import com.amitia.amitia_app.runtime.api.RuntimeListener
import com.amitia.amitia_app.runtime.api.RuntimeSnapshot
import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.api.RuntimeSubscription
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestResult
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestStore
import io.flutter.plugin.common.EventChannel
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicReference

internal class RuntimeBridgeStreamHandler(
    private val controller: RuntimeController,
    private val manifestStore: RuntimeManifestStore?,
) : EventChannel.StreamHandler {

    private val sinkRef = AtomicReference<EventChannel.EventSink?>(null)
    private val subscriptionRef = AtomicReference<RuntimeSubscription?>(null)
    private val active = AtomicBoolean(false)
    private val listener = RuntimeListener { snapshot ->
        emitSnapshot(snapshot)
    }

    override fun onListen(arguments: Any?, events: EventChannel.EventSink?) {
        if (!active.compareAndSet(false, true)) {
            sinkRef.set(events)
            return
        }
        sinkRef.set(events)
        val subscription = controller.subscribe(listener)
        subscriptionRef.set(subscription)
        emitCurrentSnapshot()
    }

    override fun onCancel(arguments: Any?) {
        if (!active.compareAndSet(true, false)) return
        subscriptionRef.getAndSet(null)?.cancel()
        sinkRef.set(null)
    }

    private fun emitCurrentSnapshot() {
        val snapshot = controller.snapshot()
        emitSnapshot(snapshot)
    }

    private fun emitSnapshot(snapshot: RuntimeSnapshot) {
        val sink = sinkRef.get() ?: return
        if (!active.get()) return
        try {
            val manifest = manifestStore?.read()
            val runtimeInstalled = manifest is RuntimeManifestResult.Success
            val runtimeAvailable = snapshot.state == RuntimeState.READY ||
                    snapshot.state == RuntimeState.DEGRADED
            val mapped = RuntimeBridgeSnapshotMapper.toBridgeSnapshot(
                snapshot = snapshot,
                manifest = (manifest as? RuntimeManifestResult.Success)?.manifest,
                runtimeInstalled = runtimeInstalled,
                runtimeAvailable = runtimeAvailable,
            )
            sink.success(mapped)
        } catch (_: Exception) {
        }
    }

    fun detach() {
        active.set(false)
        subscriptionRef.getAndSet(null)?.cancel()
        sinkRef.set(null)
    }
}
