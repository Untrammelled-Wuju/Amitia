package com.amitia.amitia_app.runtime.proot.internal

internal fun interface SessionIdGenerator { fun generate(): String }
internal class UuidSessionIdGenerator : SessionIdGenerator {
    override fun generate(): String = java.util.UUID.randomUUID().toString()
}