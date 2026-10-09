package com.amitia.amitia_app.notifications

import android.app.NotificationManager
import android.content.Context
import android.graphics.Color
import android.graphics.drawable.Icon
import android.os.Build
import android.os.Bundle
import androidx.core.app.NotificationCompat
import com.amitia.amitia_app.R

/**
 * Best-effort OriginOS 5/6 Origin Island notification extras.
 *
 * Protocol based on the publicly documented notification.superx.* schema.
 * Vivo may restrict individual scenes to enrolled/approved applications;
 * a valid notification does not prove its island card will be displayed.
 * The same ordinary progress notification remains available as fallback.
 */
internal object VivoOriginIslandTaskCompat {
    private const val SCENE = "TRAIN"
    private const val MIN_ORIGIN_ISLAND_API = 35

    fun eligible(): Boolean {
        val vendor = (Build.MANUFACTURER.orEmpty() + " " + Build.BRAND.orEmpty()).lowercase()
        return Build.VERSION.SDK_INT >= MIN_ORIGIN_ISLAND_API &&
            (vendor.contains("vivo") || vendor.contains("iqoo"))
    }

    fun attach(
        context: Context,
        builder: NotificationCompat.Builder,
        title: String,
        summary: String,
        progress: Int,
        completed: Boolean,
        timeLabel: String,
    ) {
        if (!eligible()) return
        registerScene(context)
        val icon = Icon.createWithResource(context, R.drawable.ic_notification)
        val percent = if (completed) 100 else progress.coerceIn(0, 100)
        val heading = "$title · $timeLabel"
        val state = if (completed) "已完成 · 100%" else "执行中 · $percent%"
        val inProgress = !completed && percent in 1..99
        val base = Bundle().apply {
            putParcelable("notification.superx.baseInfos.icon", icon)
            putCharSequence("notification.superx.baseInfos.title", heading.take(90))
            putCharSequence("notification.superx.baseInfos.content", summary.take(120))
            putString("notification.superx.baseInfos.subText", state)
            putInt("notification.superx.baseInfos.subInfo", 1)
        }
        val capsule = Bundle().apply {
            putInt("notification.superx.capsule.state", 1)
            putCharSequence("notification.superx.capsule.content", heading.take(40))
            putParcelable("notification.superx.capsule.icon", icon)
        }
        val infos = Bundle().apply {
            if (inProgress) {
                putInt("notification.superx.infos.progress", percent)
                putInt("notification.superx.infos.progressColor", Color.WHITE)
                putParcelableArrayList(
                    "notification.superx.infos.nodeIcon",
                    arrayListOf(icon, icon),
                )
                putParcelable("notification.superx.infos.indicatorIcon", icon)
                putInt("notification.superx.infos.indicatorLoc", 1)
            } else {
                putString("notification.superx.infos.describe", heading.take(90))
                putString("notification.superx.infos.coreInfo", state)
                putParcelable("notification.superx.infos.image", icon)
            }
        }
        val shorts = Bundle().apply {
            putString("notification.superx.shortInfos.describeShort", heading.take(90))
            putString("notification.superx.shortInfos.coreInfoShort", state)
            putParcelable("notification.superx.shortInfos.image", icon)
        }
        val left = Bundle().apply {
            putString("island.superx.leftInfo.content", heading.take(36))
            putParcelable("island.superx.leftInfo.icon", icon)
        }
        val right = Bundle().apply {
            if (inProgress) {
                putInt("island.superx.rightInfo.progressValue", percent)
                putInt("island.superx.rightInfo.progressState", 0)
                putInt("island.superx.rightInfo.progressColor", Color.WHITE)
            } else {
                putString("island.superx.rightInfo.content", state)
                putParcelable("island.superx.rightInfo.icon", icon)
            }
        }
        val island = Bundle().apply {
            putInt("island.superx.leftTemplate", 1)
            putInt("island.superx.rightTemplate", if (inProgress) 2 else 4)
            putBundle("island.superx.leftInfo", left)
            putBundle("island.superx.rightInfo", right)
        }
        builder.addExtras(Bundle().apply {
            putInt("notification.superx.operation", 0)
            putBoolean("notification.superx.showNotify", true)
            putInt("notification.superx.template", if (inProgress) 2 else 1)
            putString("notification.superx.scene", SCENE)
            putInt("notification.superx.changedRecord", 0)
            putBundle("notification.superx.baseInfos", base)
            putBundle("notification.superx.capsule", capsule)
            putBundle("notification.superx.infos", infos)
            putBundle("notification.superx.shortInfos", shorts)
            putBundle("notification.superx.island", island)
        })
    }

    private fun registerScene(context: Context) {
        runCatching {
            val manager = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
            val method = NotificationManager::class.java.getMethod(
                "setSuperXInfosSceneList",
                MutableList::class.java,
                MutableList::class.java,
                MutableList::class.java,
                MutableList::class.java,
            )
            method.invoke(
                manager,
                arrayListOf(SCENE),
                arrayListOf("true"),
                arrayListOf(context.packageName),
                arrayListOf("true"),
            )
        }
    }
}
