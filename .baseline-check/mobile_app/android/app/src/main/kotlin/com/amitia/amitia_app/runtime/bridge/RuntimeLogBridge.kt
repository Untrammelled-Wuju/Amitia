package com.amitia.amitia_app.runtime.bridge

import io.flutter.plugin.common.EventChannel
import java.util.ArrayDeque
import java.util.concurrent.CopyOnWriteArrayList

class RuntimeLogBridge : EventChannel.StreamHandler, RuntimeLogCallback {
    private val listeners = CopyOnWriteArrayList<LogListener>()
    private val bufferedLogs = ArrayDeque<Map<String, Any>>()
    private val logLock = Any()

    interface LogListener {
        fun onLog(level: String, message: String)
    }

    fun addListener(listener: LogListener) {
        listeners.addIfAbsent(listener)
    }

    fun removeListener(listener: LogListener) {
        listeners.remove(listener)
    }

    override fun onLog(level: String, message: String) {
        val entry = mapOf(
            "level" to level,
            "message" to message,
            "timestamp" to System.currentTimeMillis(),
        )
        synchronized(logLock) {
            if (bufferedLogs.size >= MAX_BUFFERED_LOGS) {
                bufferedLogs.removeFirst()
            }
            bufferedLogs.addLast(entry)
        }
        for (listener in listeners) {
            try {
                listener.onLog(level, message)
            } catch (_: Throwable) {}
        }
    }

    override fun onListen(arguments: Any?, events: EventChannel.EventSink?) {
        val sink = events ?: return
        val listener = object : LogListener {
            override fun onLog(level: String, message: String) {
                try {
                    sink.success(mapOf(
                        "level" to level,
                        "message" to message,
                        "timestamp" to System.currentTimeMillis()
                    ))
                } catch (_: Throwable) {}
            }
        }
        addListener(listener)
        RuntimeLogCallback.instance = this
        val entries = synchronized(logLock) { bufferedLogs.toList() }
        for (entry in entries) {
            try {
                sink.success(entry)
            } catch (_: Throwable) {
            }
        }
    }

    override fun onCancel(arguments: Any?) {
        RuntimeLogCallback.instance = null
    }

    companion object {
        const val EVENT_CHANNEL = "com.amitia.runtime/logs"
        private const val MAX_BUFFERED_LOGS = 200

        @Volatile
        private var instance: RuntimeLogBridge? = null

        fun getInstance(): RuntimeLogBridge {
            return instance ?: synchronized(this) {
                instance ?: RuntimeLogBridge().also { instance = it }
            }
        }
    }
}
