package com.amitia.amitia_app.runtime.service

import android.content.Context
import android.content.Intent
import android.os.Build

object BackgroundKeepAlive {
    @Volatile private var runtimeForeground = false
    @Volatile private var standaloneForeground = false
    @Volatile var lastError: String? = null
        private set
    val active: Boolean get() = runtimeForeground || standaloneForeground
    val usingRuntime: Boolean get() = runtimeForeground

    fun enabled(context: Context): Boolean = preferences(context).getBoolean("enabled", false)
    private fun preferences(context: Context) = context.getSharedPreferences("amitia.background.keepalive", Context.MODE_PRIVATE)

    @Synchronized fun setEnabled(context: Context, enabled: Boolean) {
        check(preferences(context).edit().putBoolean("enabled", enabled).commit()) { "后台保活设置保存失败" }
        reconcile(context)
    }

    @Synchronized fun restore(context: Context) {
        runCatching { reconcile(context) }.onFailure { lastError = it.message ?: "后台保活启动失败" }
    }

    private fun reconcile(context: Context) {
        lastError = null
        val intent = Intent(context, BackgroundKeepAliveService::class.java)
        if (!enabled(context) || runtimeForeground) {
            context.stopService(intent)
        } else if (!standaloneForeground) {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) context.startForegroundService(intent)
            else context.startService(intent)
        }
    }

    @Synchronized fun runtimeStarted(context: Context) {
        runtimeForeground = true
        runCatching { context.stopService(Intent(context, BackgroundKeepAliveService::class.java)) }
    }

    @Synchronized fun runtimeStopped(context: Context) {
        runtimeForeground = false
        restore(context)
    }

    fun standaloneStarted() {
        standaloneForeground = true
        lastError = null
    }

    fun standaloneStopped() { standaloneForeground = false }
    fun standaloneFailed(message: String) {
        standaloneForeground = false
        lastError = message
    }
}
