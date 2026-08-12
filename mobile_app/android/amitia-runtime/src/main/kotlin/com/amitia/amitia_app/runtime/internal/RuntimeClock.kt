package com.amitia.amitia_app.runtime.internal

interface RuntimeClock {
    fun nowEpochMillis(): Long
}

object SystemRuntimeClock : RuntimeClock {
    override fun nowEpochMillis(): Long = System.currentTimeMillis()
}
