package com.amitia.amitia_app.runtime.abi

interface AndroidAbiProvider {
    fun snapshot(): RuntimeAbiSnapshot
}