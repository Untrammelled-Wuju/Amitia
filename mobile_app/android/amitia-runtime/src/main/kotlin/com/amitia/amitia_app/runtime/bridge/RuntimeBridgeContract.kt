package com.amitia.amitia_app.runtime.bridge

object RuntimeBridgeContract {
    const val METHOD_CHANNEL = "com.amitia.runtime/bridge"
    const val EVENT_CHANNEL = "com.amitia.runtime/events"

    const val METHOD_SNAPSHOT = "runtime.snapshot"
    const val METHOD_START = "runtime.start"
    const val METHOD_STOP = "runtime.stop"
    const val METHOD_INSTALL = "runtime.install"
    const val METHOD_VERIFY = "runtime.verify"
    const val METHOD_REPAIR = "runtime.repair"
    const val METHOD_MANIFEST_SUMMARY = "runtime.manifestSummary"
    const val METHOD_GET_BACKEND_CONNECTION = "runtime.getBackendConnection"

    const val SCHEMA_VERSION = 1
}
