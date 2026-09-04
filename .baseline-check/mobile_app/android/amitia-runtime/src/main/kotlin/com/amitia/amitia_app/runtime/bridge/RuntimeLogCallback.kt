package com.amitia.amitia_app.runtime.bridge

interface RuntimeLogCallback {
    fun onLog(level: String, message: String)

    companion object {
        @Volatile
        var instance: RuntimeLogCallback? = null
    }
}
