package com.amitia.amitia_app.runtime.internal

import com.amitia.amitia_app.runtime.api.RuntimeController
import com.amitia.amitia_app.runtime.api.RuntimeModule

internal class DefaultRuntimeModule(
    override val controller: RuntimeController = UnsupportedRuntimeController(),
    private val stateStore: RuntimeStateStore = RuntimeStateStore()
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

    private object NoopProotComponent {
        fun availability(): Any = UnsupportedOperationException("not implemented")
        fun launch(request: Any, observer: Any): Any = UnsupportedOperationException("not implemented")
        fun close() {}
    }
}
