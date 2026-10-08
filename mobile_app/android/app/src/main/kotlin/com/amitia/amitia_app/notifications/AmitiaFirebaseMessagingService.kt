package com.amitia.amitia_app.notifications

import com.google.firebase.messaging.FirebaseMessagingService
import com.google.firebase.messaging.RemoteMessage

class AmitiaFirebaseMessagingService : FirebaseMessagingService() {
    override fun onCreate() {
        super.onCreate()
        NotificationRenderer.ensureChannels(this)
    }

    override fun onMessageReceived(message: RemoteMessage) {
        NotificationRenderer.handleRemoteMessage(this, message.data)
    }

    override fun onNewToken(token: String) {
        PushBootstrap.storeToken(this, token)
        NotificationPlatformPlugin.emitToken(token)
    }
}
