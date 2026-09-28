package com.amitia.amitia_app.virtualdisplay.host

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.os.Build

class VirtualDisplayHostBinderReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != VirtualDisplayHostContract.ACTION_READY) return
        val token = intent.getStringExtra(VirtualDisplayHostContract.EXTRA_TOKEN).orEmpty()
        val container = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            intent.getParcelableExtra(
                VirtualDisplayHostContract.EXTRA_CONTAINER,
                VirtualDisplayHostBinderContainer::class.java,
            )
        } else {
            @Suppress("DEPRECATION")
            intent.getParcelableExtra(VirtualDisplayHostContract.EXTRA_CONTAINER)
        }
        val binder = container?.binder ?: return
        VirtualDisplayHostBinderRegistry.accept(context.applicationContext, binder, token)
    }
}
