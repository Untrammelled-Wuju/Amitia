package com.amitia.amitia_app.nativeprovider.notification

import android.app.NotificationManager
import android.content.Context
import android.media.AudioAttributes
import android.media.AudioManager
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.os.VibrationAttributes
import android.os.VibrationEffect
import android.os.Vibrator
import android.os.VibratorManager
import com.amitia.amitia_app.MainActivity

internal object ReplyNotificationVibration {
    private val handler = Handler(Looper.getMainLooper())
    private var active: Vibrator? = null
    private var generation = 0

    fun play(context: Context) {
        handler.post {
            if (MainActivity.currentActivity() != null) return@post
            runCatching {
                val audio = context.getSystemService(Context.AUDIO_SERVICE) as AudioManager
                val notifications = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
                if (audio.ringerMode == AudioManager.RINGER_MODE_SILENT) return@runCatching
                if (notifications.currentInterruptionFilter != NotificationManager.INTERRUPTION_FILTER_ALL) return@runCatching
                val vibrator = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
                    (context.getSystemService(Context.VIBRATOR_MANAGER_SERVICE) as VibratorManager).defaultVibrator
                } else {
                    context.getSystemService(Context.VIBRATOR_SERVICE) as Vibrator
                }
                if (!vibrator.hasVibrator()) return@runCatching
                stop()
                val token = ++generation
                active = vibrator
                val effect = VibrationEffect.createOneShot(180, VibrationEffect.DEFAULT_AMPLITUDE)
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                    vibrator.vibrate(effect, VibrationAttributes.Builder().setUsage(VibrationAttributes.USAGE_NOTIFICATION).build())
                } else {
                    vibrator.vibrate(effect, AudioAttributes.Builder().setUsage(AudioAttributes.USAGE_NOTIFICATION).build())
                }
                handler.postDelayed({ if (generation == token) active = null }, 180)
            }
        }
    }

    fun stop() {
        generation++
        runCatching { active?.cancel() }
        active = null
    }
}
