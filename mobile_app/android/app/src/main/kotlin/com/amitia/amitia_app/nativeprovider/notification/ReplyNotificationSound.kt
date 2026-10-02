package com.amitia.amitia_app.nativeprovider.notification

import android.app.NotificationManager
import android.content.Context
import android.media.AudioAttributes
import android.media.AudioManager
import android.media.Ringtone
import android.media.RingtoneManager
import android.os.Handler
import android.os.Looper
import com.amitia.amitia_app.MainActivity

internal object ReplyNotificationSound {
    private val handler = Handler(Looper.getMainLooper())
    private var active: Ringtone? = null

    fun play(context: Context) {
        handler.post {
            if (MainActivity.currentActivity() != null) return@post
            val audio = context.getSystemService(Context.AUDIO_SERVICE) as AudioManager
            val notifications = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
            if (audio.ringerMode != AudioManager.RINGER_MODE_NORMAL || audio.getStreamVolume(AudioManager.STREAM_NOTIFICATION) == 0) return@post
            if (notifications.currentInterruptionFilter != NotificationManager.INTERRUPTION_FILTER_ALL) return@post
            runCatching {
                stop()
                val uri = RingtoneManager.getActualDefaultRingtoneUri(context, RingtoneManager.TYPE_NOTIFICATION) ?: return@runCatching
                val ringtone = RingtoneManager.getRingtone(context, uri) ?: return@runCatching
                ringtone.audioAttributes = AudioAttributes.Builder().setUsage(AudioAttributes.USAGE_NOTIFICATION).setContentType(AudioAttributes.CONTENT_TYPE_SONIFICATION).build()
                active = ringtone
                ringtone.play()
                handler.postDelayed({
                    if (active === ringtone) stop()
                }, 1500)
            }
        }
    }

    fun stop() {
        active?.stop()
        active = null
    }
}
