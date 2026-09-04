package com.amitia.amitia_app.runtime.internal

import java.util.UUID

internal interface RuntimeIdGenerator {
    fun nextOperationId(): String
}

internal object UuidRuntimeIdGenerator : RuntimeIdGenerator {
    override fun nextOperationId(): String = UUID.randomUUID().toString()
}
