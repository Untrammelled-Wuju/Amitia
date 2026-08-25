package com.amitia.amitia_app.runtime.service.internal

import android.content.ComponentName
import android.content.Context
import android.content.ServiceConnection
import android.os.IBinder
import com.amitia.amitia_app.runtime.service.RuntimeServiceBinder
import com.amitia.amitia_app.runtime.service.RuntimeServiceEndpoint
import java.util.concurrent.atomic.AtomicReference

internal class RuntimeServiceConnection(
    private val onConnected: ((RuntimeServiceEndpoint) -> Unit)? = null,
    private val onDisconnected: (() -> Unit)? = null
) : ServiceConnection {

    private val endpointRef = AtomicReference<RuntimeServiceEndpoint?>(null)

    override fun onServiceConnected(name: ComponentName?, service: IBinder?) {
        val binder = service as? RuntimeServiceBinder
        val endpoint = binder?.endpoint
        endpointRef.set(endpoint)
        if (endpoint != null) {
            onConnected?.invoke(endpoint)
        } else {
            onDisconnected?.invoke()
        }
    }

    override fun onServiceDisconnected(name: ComponentName?) {
        clearEndpoint()
        onDisconnected?.invoke()
    }

    override fun onBindingDied(name: ComponentName?) {
        clearEndpoint()
        onDisconnected?.invoke()
    }

    override fun onNullBinding(name: ComponentName?) {
        clearEndpoint()
        onDisconnected?.invoke()
    }

    fun endpoint(): RuntimeServiceEndpoint? = endpointRef.get()

    fun clearEndpoint() {
        endpointRef.set(null)
    }

    companion object {
        fun bind(context: Context, connection: RuntimeServiceConnection): Boolean {
            return try {
                val intent = android.content.Intent(
                    context,
                    com.amitia.amitia_app.runtime.service.RuntimeService::class.java
                )
                context.bindService(intent, connection, Context.BIND_AUTO_CREATE)
            } catch (_: Exception) {
                connection.clearEndpoint()
                false
            }
        }

        fun unbind(context: Context, connection: RuntimeServiceConnection): Boolean {
            connection.clearEndpoint()
            return try {
                context.unbindService(connection)
                true
            } catch (_: Exception) {
                false
            }
        }
    }
}
