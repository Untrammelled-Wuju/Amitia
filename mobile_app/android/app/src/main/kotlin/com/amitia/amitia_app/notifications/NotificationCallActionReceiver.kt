package com.amitia.amitia_app.notifications

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import androidx.core.app.NotificationManagerCompat

class NotificationCallActionReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val callId = intent.getStringExtra("callId").orEmpty()
        val conversationId = intent.getStringExtra("conversationId").orEmpty()
        val deepLink = intent.getStringExtra("deepLink").orEmpty()
        if (callId.isBlank()) return
        val interaction = Intent()
            .putExtra("amitia.deepLink", deepLink)
            .putExtra("amitia.conversationId", conversationId)
            .putExtra("amitia.callAction", "decline")
        NotificationPlatformPlugin.handleIntent(context, interaction)
        NotificationManagerCompat.from(context)
            .cancel(("call:$callId").hashCode() and 0x7fffffff)
    }
}
