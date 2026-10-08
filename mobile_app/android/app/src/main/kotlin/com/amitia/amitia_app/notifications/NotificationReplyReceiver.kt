package com.amitia.amitia_app.notifications

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.net.Uri
import androidx.core.app.RemoteInput
import com.amitia.amitia_app.MainActivity

class NotificationReplyReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val reply = RemoteInput.getResultsFromIntent(intent)
            ?.getCharSequence(NotificationRenderer.REPLY_KEY)
            ?.toString()
            ?.trim()
            .orEmpty()
        if (reply.isEmpty()) return
        val conversationId = intent.getStringExtra("conversationId").orEmpty()
        val deepLink = intent.getStringExtra("deepLink")
            ?.takeIf { it.isNotBlank() }
            ?: "amitia://chat/$conversationId"
        val launch = Intent(context, MainActivity::class.java)
            .setAction(Intent.ACTION_VIEW)
            .setData(Uri.parse(deepLink))
            .putExtra("amitia.deepLink", deepLink)
            .putExtra("amitia.reply", reply)
            .putExtra("amitia.conversationId", conversationId)
            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP)
        // MainActivity.onCreate/onNewIntent is the single interaction entry.
        // Dispatching here as well would submit the same inline reply twice
        // whenever an already-running Activity receives this Intent.
        context.startActivity(launch)
    }
}
