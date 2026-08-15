package com.amitia.amitia_app.nativeprovider.shizuku

import android.content.ServiceConnection
import android.os.IBinder
import rikka.shizuku.Shizuku
import java.util.concurrent.atomic.AtomicReference

enum class ShizukuServiceState {
    UNAVAILABLE,
    PERMISSION_REQUIRED,
    BINDING,
    READY,
    DEAD,
    ERROR,
}

object ShizukuCommandServiceHolder {
    private val proxyRef = AtomicReference<IPrivilegedCommandService?>(null)
    private val stateRef = AtomicReference(ShizukuServiceState.UNAVAILABLE)

    @Volatile
    private var connection: ServiceConnection? = null

    @Volatile
    private var bindingInProgress = false

    private var serviceConnectedListener: (() -> Unit)? = null

    fun currentState(): ShizukuServiceState = stateRef.get()

    fun setServiceConnectedListener(listener: (() -> Unit)?) {
        serviceConnectedListener = listener
    }

    fun currentService(): IPrivilegedCommandService? {
        return proxyRef.get()
    }

    fun bindService(): Boolean {
        if (proxyRef.get() != null) return true
        if (!Shizuku.pingBinder()) {
            stateRef.set(ShizukuServiceState.UNAVAILABLE)
            return false
        }

        synchronized(this) {
            if (proxyRef.get() != null) return true
            if (bindingInProgress) return true

            if (Shizuku.checkSelfPermission() != android.content.pm.PackageManager.PERMISSION_GRANTED) {
                stateRef.set(ShizukuServiceState.PERMISSION_REQUIRED)
                return false
            }

            bindingInProgress = true
            stateRef.set(ShizukuServiceState.BINDING)
        }

        return try {
            val args = ShizukuCommandService.createArgs()
            val conn = object : ServiceConnection {
                override fun onServiceConnected(name: android.content.ComponentName?, binder: IBinder?) {
                    binder?.let {
                        val svc = IPrivilegedCommandService.Stub.asInterface(it)
                        proxyRef.set(svc)
                        stateRef.set(ShizukuServiceState.READY)
                        try {
                            it.linkToDeath({
                                handleBinderDeath()
                            }, 0)
                        } catch (_: Exception) {}
                    } ?: run {
                        stateRef.set(ShizukuServiceState.ERROR)
                    }
                    bindingInProgress = false
                    serviceConnectedListener?.invoke()
                }

                override fun onServiceDisconnected(name: android.content.ComponentName?) {
                    proxyRef.set(null)
                    stateRef.set(ShizukuServiceState.DEAD)
                    connection = null
                    bindingInProgress = false
                }

                override fun onBindingDied(name: android.content.ComponentName?) {
                    handleBinderDeath()
                }
            }

            Shizuku.bindUserService(args, conn)
            connection = conn
            true
        } catch (e: Exception) {
            stateRef.set(ShizukuServiceState.ERROR)
            bindingInProgress = false
            false
        }
    }

    private fun handleBinderDeath() {
        proxyRef.set(null)
        stateRef.set(ShizukuServiceState.DEAD)
        connection = null
        bindingInProgress = false
    }

    fun unbindService() {
        connection?.let {
            try {
                Shizuku.unbindUserService(ShizukuCommandService.createArgs(), it, true)
            } catch (_: Exception) {}
        }
        connection = null
        proxyRef.set(null)
        stateRef.set(ShizukuServiceState.DEAD)
        bindingInProgress = false
    }

    fun onServiceDestroyed(instance: ShizukuCommandService) {
        proxyRef.set(null)
        stateRef.set(ShizukuServiceState.DEAD)
    }
}
