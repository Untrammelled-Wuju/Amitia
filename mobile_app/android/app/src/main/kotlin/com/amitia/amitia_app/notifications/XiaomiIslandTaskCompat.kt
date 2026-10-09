package com.amitia.amitia_app.notifications

import android.content.Context
import android.graphics.drawable.Icon
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.provider.Settings
import android.util.Log
import androidx.core.app.NotificationCompat
import com.amitia.amitia_app.R
import org.json.JSONObject

internal object XiaomiIslandTaskCompat {
    private const val TAG = "AmitiaXiaomiIsland"
    private const val ICON_KEY = "miui.focus.pic_agent"

    fun supported(context: Context): Boolean =
        Build.VERSION.SDK_INT >= Build.VERSION_CODES.O &&
            runCatching {
                Settings.System.getInt(
                    context.contentResolver, "notification_focus_protocol", 0,
                ) >= 3
            }.getOrDefault(false)

    fun focusPermission(context: Context): Boolean =
        supported(context) && runCatching {
            context.contentResolver.call(
                Uri.parse("content://miui.statusbar.notification.public"),
                "canShowFocus",
                null,
                Bundle().apply { putString("package", context.packageName) },
            )?.getBoolean("canShowFocus", false) == true
        }.getOrDefault(false)

    fun attach(
        context: Context,
        builder: NotificationCompat.Builder,
        title: String,
        summary: String,
        progress: Int,
        revision: Long,
        terminal: Boolean,
        timeLabel: String,
    ) {
        if (!supported(context)) return
        val percent = progress.coerceIn(0, 100)
        val percentText = "$percent%"
        val heading = "$title · $timeLabel"
        val icon = JSONObject()
            .put("type", 1)
            .put("pic", ICON_KEY)
        val progressInfo = JSONObject()
            .put("progress", percent)
            .put("colorProgress", "#FFFFFF")
            .put("colorProgressEnd", "#FFFFFF")
        val paramIsland = JSONObject()
            .put("islandProperty", 1)
            .put("islandTimeout", if (terminal) 120 else 3600)
            .put("dismissIsland", false)
            .put("highlightColor", "#FFFFFF")
            .put(
                "bigIslandArea",
                JSONObject()
                    .put(
                        "imageTextInfoLeft",
                        JSONObject()
                            .put("type", 1)
                            .put("picInfo", icon)
                            .put(
                                "textInfo",
                                JSONObject()
                                    .put("frontTitle", heading.take(34))
                                    .put("title", if (terminal) "已完成" else percentText)
                                    .put("content", if (terminal) "100%" else "执行中")
                                    .put("showHighlightColor", false),
                            ),
                    )
                    .put(
                        "progressTextInfo",
                        JSONObject()
                            .put(
                                "progressInfo",
                                JSONObject()
                                    .put("progress", percent)
                                    .put("colorReach", "#FFFFFF")
                                    .put("colorUnReach", "#3A3A3A"),
                            )
                            .put(
                                "textInfo",
                                JSONObject()
                                    .put("frontTitle", "执行进度")
                                    .put("title", percentText),
                            ),
                    ),
            )
            .put(
                "smallIslandArea",
                JSONObject().put(
                    "combinePicInfo",
                    JSONObject()
                        .put("picInfo", icon)
                        .put(
                            "progressInfo",
                            JSONObject()
                                .put("progress", percent)
                                .put("colorReach", "#FFFFFF")
                                .put("colorUnReach", "#444444")
                                .put("isCCW", false),
                        ),
                ),
            )
        val params = JSONObject().put(
            "param_v2",
            JSONObject()
                .put("protocol", 1)
                .put("business", "agent_task")
                .put("timeout", if (terminal) 3 else 720)
                .put("cancel", false)
                .put("islandFirstFloat", false)
                .put("enableFloat", terminal)
                .put("updatable", true)
                .put("sequence", revision.coerceAtLeast(1L))
                .put("aodTitle", if (terminal) "$heading · 已完成 100%" else "$heading · $percentText")
                .put("ticker", if (terminal) "$heading · 已完成 100%" else "$heading · $percentText")
                .put("param_island", paramIsland)
                .put(
                    "baseInfo",
                    JSONObject()
                        .put("type", 1)
                        .put("title", heading.take(35))
                        .put("content", summary.take(100)),
                )
                .put("picInfo", icon)
                .put("progressInfo", progressInfo)
                .put(
                    "bgInfo",
                    JSONObject()
                        .put("type", 1)
                        .put("colorBg", "#000000"),
                ),
        )
        runCatching {
            val pictures = Bundle().apply {
                putParcelable(ICON_KEY, Icon.createWithResource(context, R.mipmap.ic_launcher))
            }
            builder.addExtras(Bundle().apply {
                putBundle("miui.focus.pics", pictures)
                putString("miui.focus.param", params.toString())
            })
        }.onFailure { Log.w(TAG, "Unable to attach system island data", it) }
    }
}
