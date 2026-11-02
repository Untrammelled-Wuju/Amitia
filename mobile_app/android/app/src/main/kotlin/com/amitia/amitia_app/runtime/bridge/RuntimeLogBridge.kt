package com.amitia.amitia_app.runtime.bridge

import android.os.Handler
import android.os.Looper
import android.util.Log
import com.amitia.amitia_app.runtime.logging.DailySizeRotatingLog
import io.flutter.plugin.common.EventChannel
import java.io.File
import java.text.SimpleDateFormat
import java.util.ArrayDeque
import java.util.Date
import java.util.Locale
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.atomic.AtomicReference

class RuntimeLogBridge(
    logDirectory: File?,
) : EventChannel.StreamHandler, RuntimeLogCallback {
    private val listeners = CopyOnWriteArrayList<LogListener>()
    private val bufferedLogs = ArrayDeque<Map<String, Any>>()
    private val logLock = Any()
    private val mainHandler = Handler(Looper.getMainLooper())
    private val sinkRef = AtomicReference<EventChannel.EventSink?>(null)
    private val timestampFormat = SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss.SSSXXX", Locale.US)
    private val fileLog = createFileLog(logDirectory)

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
        persist(level, message)
        emitToSink(entry)
        for (listener in listeners) {
            try {
                listener.onLog(level, message)
            } catch (originalError39: Throwable) {
              originalError39.printStackTrace()
            }
        }
    }

    override fun onListen(arguments: Any?, events: EventChannel.EventSink?) {
        val sink = events ?: return
        sinkRef.set(sink)
        RuntimeLogCallback.instance = this
        val entries = synchronized(logLock) { bufferedLogs.toList() }
        for (entry in entries) {
            emitToSink(entry)
        }
    }

    override fun onCancel(arguments: Any?) {
        sinkRef.set(null)
        RuntimeLogCallback.instance = null
    }

    private fun emitToSink(entry: Map<String, Any>) {
        val sink = sinkRef.get() ?: return
        mainHandler.post {
            if (sinkRef.get() !== sink) return@post
            try {
                sink.success(entry)
            } catch (error: Throwable) {
                Log.e(EVENT_CHANNEL, "Runtime log event delivery failed", error)
                try {
                    sink.error("RUNTIME_LOG_EVENT_FAILED", error.toString(), Log.getStackTraceString(error))
                } catch (deliveryError: Throwable) {
                    Log.e(EVENT_CHANNEL, "Runtime log error delivery failed", deliveryError)
                }
            }
        }
    }

    private fun persist(level: String, message: String) {
        val writer = fileLog ?: return
        try {
            val timestamp = synchronized(timestampFormat) { timestampFormat.format(Date()) }
            writer.write("[$timestamp] [$level] $message\n")
        } catch (error: Throwable) {
            Log.e(EVENT_CHANNEL, "Runtime log file write failed", error)
        }
    }

    private fun createFileLog(directory: File?): DailySizeRotatingLog? {
        if (directory == null) return null
        return try {
            DailySizeRotatingLog(directory, "mobile")
        } catch (error: Throwable) {
            Log.e(EVENT_CHANNEL, "Runtime log file initialization failed: ${directory.absolutePath}", error)
            null
        }
    }

    companion object {
        const val EVENT_CHANNEL = "com.amitia.runtime/logs"
        private const val MAX_BUFFERED_LOGS = 200

        @Volatile
        private var instance: RuntimeLogBridge? = null

        fun getInstance(logDirectory: File): RuntimeLogBridge {
            return instance ?: synchronized(this) {
                instance ?: RuntimeLogBridge(logDirectory).also { instance = it }
            }
        }
    }
}
