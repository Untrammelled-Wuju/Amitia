package com.amitia.amitia_app.runtime.internal

internal interface RuntimeClock {
    fun nowEpochMillis(): Long
}

internal object SystemRuntimeClock : RuntimeClock {
    override fun nowEpochMillis(): Long = System.currentTimeMillis()
}
