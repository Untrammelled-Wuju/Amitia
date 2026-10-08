package com.amitia.amitia_app.notifications

import android.content.Context
import com.amitia.amitia_app.BuildConfig
import com.google.firebase.FirebaseApp
import com.google.firebase.FirebaseOptions
import com.google.firebase.messaging.FirebaseMessaging

object PushBootstrap {
    private const val PREFS = "amitia_notification_push"
    private const val TOKEN = "fcm_token"

    fun initialize(context: Context): Boolean {
        if (FirebaseApp.getApps(context).isNotEmpty()) return true
        val applicationId = BuildConfig.AMITIA_FCM_APPLICATION_ID.trim()
        val apiKey = BuildConfig.AMITIA_FCM_API_KEY.trim()
        val projectId = BuildConfig.AMITIA_FCM_PROJECT_ID.trim()
        val senderId = BuildConfig.AMITIA_FCM_SENDER_ID.trim()
        if (applicationId.isEmpty() || apiKey.isEmpty() || projectId.isEmpty() || senderId.isEmpty()) {
            return false
        }
        val options = FirebaseOptions.Builder()
            .setApplicationId(applicationId)
            .setApiKey(apiKey)
            .setProjectId(projectId)
            .setGcmSenderId(senderId)
            .build()
        FirebaseApp.initializeApp(context, options)
        return FirebaseApp.getApps(context).isNotEmpty()
    }

    fun refreshToken(context: Context, callback: (String?) -> Unit) {
        if (!initialize(context)) {
            callback(currentToken(context))
            return
        }
        FirebaseMessaging.getInstance().token.addOnCompleteListener { task ->
            val token = if (task.isSuccessful) task.result?.trim().orEmpty() else ""
            if (token.isNotEmpty()) {
                storeToken(context, token)
                NotificationPlatformPlugin.emitToken(token)
                callback(token)
            } else {
                callback(currentToken(context))
            }
        }
    }

    fun storeToken(context: Context, token: String) {
        val normalized = token.trim()
        if (normalized.isEmpty()) return
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            .edit()
            .putString(TOKEN, normalized)
            .apply()
    }

    fun currentToken(context: Context): String? {
        return context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            .getString(TOKEN, null)
            ?.trim()
            ?.takeIf { it.isNotEmpty() }
    }

    fun configured(): Boolean {
        return BuildConfig.AMITIA_FCM_APPLICATION_ID.isNotBlank() &&
            BuildConfig.AMITIA_FCM_API_KEY.isNotBlank() &&
            BuildConfig.AMITIA_FCM_PROJECT_ID.isNotBlank() &&
            BuildConfig.AMITIA_FCM_SENDER_ID.isNotBlank()
    }
}
