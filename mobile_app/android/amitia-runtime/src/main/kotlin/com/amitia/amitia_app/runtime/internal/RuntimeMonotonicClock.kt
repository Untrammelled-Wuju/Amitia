package com.amitia.amitia_app.runtime.internal

internal fun interface RuntimeMonotonicClock {
    fun nowNanos(): Long
}

internal object SystemRuntimeMonotonicClock : RuntimeMonotonicClock {
    override fun nowNanos(): Long = System.nanoTime()
}
