package com.amitia.amitia_app.runtime.internal

import com.amitia.amitia_app.runtime.abi.RuntimeAbiGate
import com.amitia.amitia_app.runtime.api.RuntimeController
import com.amitia.amitia_app.runtime.api.RuntimeModule
import com.amitia.amitia_app.runtime.install.RuntimeInstaller
import com.amitia.amitia_app.runtime.service.RuntimeServiceHost

internal class DefaultRuntimeModule(
    override val controller: RuntimeController = UnsupportedRuntimeController(),
    override val runtimeInstaller: RuntimeInstaller,
    private val stateStore: RuntimeStateStore = RuntimeStateStore(),
    private val abiGate: RuntimeAbiGate? = null,
    private val serviceHost: RuntimeServiceHost? = null
) : RuntimeModule {

    private val closed = java.util.concurrent.atomic.AtomicBoolean(false)

    override fun prootComponent(): Any {
        return NoopProotComponent
    }

    override fun close() {
        if (closed.compareAndSet(false, true)) {
            stateStore.close()
        }
    }

    internal fun abiGate(): RuntimeAbiGate? = abiGate
    internal fun serviceHost(): RuntimeServiceHost? = serviceHost
    internal fun stateStore(): RuntimeStateStore = stateStore

    private object NoopProotComponent {
        fun availability(): Any = UnsupportedOperationException("not implemented")
        fun launch(request: Any, observer: Any): Any = UnsupportedOperationException("not implemented")
        fun close() {}
    }
}
