package com.amitia.amitia_app.runtime.abi

interface RuntimeAbiGate {
    fun evaluate(): RuntimeAbiStatus
    fun isSupported(): Boolean
}