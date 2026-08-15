package com.amitia.amitia_app.nativeprovider.shizuku

import android.content.ServiceConnection
import android.os.IBinder
import rikka.shizuku.Shizuku

object ShizukuCommandServiceHolder {
    @Volatile
    var service: ShizukuCommandService? = null
        private set

    private var connection: ServiceConnection? = null

    fun bindService(): Boolean {
        if (service != null) return true
        if (!Shizuku.pingBinder()) return false

        return try {
            if (Shizuku.checkSelfPermission() != android.content.pm.PackageManager.PERMISSION_GRANTED) {
                return false
            }

            val args = ShizukuCommandService.createArgs()
            val conn = object : ServiceConnection {
                override fun onServiceConnected(name: android.content.ComponentName?, binder: IBinder?) {
                    binder?.let {
                        service = (it as? ShizukuCommandService)
                    }
                }

                override fun onServiceDisconnected(name: android.content.ComponentName?) {
                    service = null
                }

                override fun onBindingDied(name: android.content.ComponentName?) {
                    service = null
                    connection = null
                }
            }

            Shizuku.bindUserService(args, conn)
            connection = conn
            true
        } catch (e: Exception) {
            false
        }
    }

    fun unbindService() {
        connection?.let {
            try {
                Shizuku.unbindUserService(ShizukuCommandService.createArgs(), it, true)
            } catch (_: Exception) {}
        }
        connection = null
        service = null
    }
}
