package com.amitia.amitia_app.nativeprovider.shizuku

import android.content.ServiceConnection
import android.os.IBinder
import rikka.shizuku.Shizuku
import java.util.concurrent.CopyOnWriteArrayList
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

    private val serviceConnectedListeners = CopyOnWriteArrayList<() -> Unit>()

    fun currentState(): ShizukuServiceState = stateRef.get()

    fun addServiceConnectedListener(listener: () -> Unit) {
        serviceConnectedListeners.addIfAbsent(listener)
    }

    fun removeServiceConnectedListener(listener: () -> Unit) {
        serviceConnectedListeners.remove(listener)
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
                            val recipient = IBinder.DeathRecipient {
                                handleBinderDeath()
                            }
                            deathRecipient = recipient
                            it.linkToDeath(recipient, 0)
                        } catch (_: Exception) {}
                    } ?: run {
                        stateRef.set(ShizukuServiceState.ERROR)
                    }
                    bindingInProgress = false
                    notifyServiceStateChanged()
                }

                override fun onServiceDisconnected(name: android.content.ComponentName?) {
                    proxyRef.set(null)
                    stateRef.set(ShizukuServiceState.DEAD)
                    connection = null
                    bindingInProgress = false
                    notifyServiceStateChanged()
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
        notifyServiceStateChanged()
    }

    private fun notifyServiceStateChanged() {
        serviceConnectedListeners.forEach { listener ->
            try {
                listener.invoke()
            } catch (_: Exception) {
            }
        }
        serviceConnectedListeners.clear()
    }

    private var deathRecipient: IBinder.DeathRecipient? = null

    fun unbindService() {
        runDestroyTransaction()
    }

    private fun runDestroyTransaction() {
        val svc = proxyRef.getAndSet(null) ?: return
        try {
            deathRecipient?.let { recipient ->
                try {
                    svc.asBinder().unlinkToDeath(recipient, 0)
                } catch (_: Exception) {}
            }
            deathRecipient = null
            try {
                svc.destroy()
            } catch (_: Exception) {}
        } finally {
            connection?.let { conn ->
                try {
                    Shizuku.unbindUserService(ShizukuCommandService.createArgs(), conn, true)
                } catch (_: Exception) {}
            }
            connection = null
            stateRef.set(ShizukuServiceState.DEAD)
            bindingInProgress = false
            notifyServiceStateChanged()
        }
    }

    fun onServiceDestroyed(instance: ShizukuCommandService) {
        proxyRef.set(null)
        deathRecipient = null
        connection = null
        stateRef.set(ShizukuServiceState.DEAD)
        bindingInProgress = false
        notifyServiceStateChanged()
    }
}
