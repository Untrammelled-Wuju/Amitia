package com.amitia.amitia_app.runtime.internal

import com.amitia.amitia_app.runtime.abi.RuntimeAbiGate
import com.amitia.amitia_app.runtime.api.RuntimeController
import com.amitia.amitia_app.runtime.api.RuntimeModule
import com.amitia.amitia_app.runtime.connection.BackendConnectionProvider
import com.amitia.amitia_app.runtime.install.RuntimeInstaller
import com.amitia.amitia_app.runtime.proot.ProotComponent
import com.amitia.amitia_app.runtime.service.RuntimeServiceHost

class DefaultRuntimeModule(
    override val controller: RuntimeController = UnsupportedRuntimeController(),
    override val runtimeInstaller: RuntimeInstaller,
    val backendConnectionProvider: BackendConnectionProvider,
    private val stateStore: RuntimeStateStore = RuntimeStateStore(),
    private val abiGate: RuntimeAbiGate? = null,
    private val serviceHost: RuntimeServiceHost? = null,
    private val prootComponent: ProotComponent? = null,
) : RuntimeModule {

    private val closed = java.util.concurrent.atomic.AtomicBoolean(false)

    override fun prootComponent(): Any {
        return prootComponent ?: NoopProotComponent
    }

    override fun close() {
        if (closed.compareAndSet(false, true)) {
            stateStore.close()
            prootComponent?.let { runCatching { it.close() } }
        }
    }

    fun abiGate(): RuntimeAbiGate? = abiGate
    fun serviceHost(): RuntimeServiceHost? = serviceHost
    fun stateStore(): RuntimeStateStore = stateStore

    private object NoopProotComponent {
        fun availability(): Any = UnsupportedOperationException("not implemented")
        fun launch(request: Any, observer: Any): Any = UnsupportedOperationException("not implemented")
        fun close() {}
    }
}
