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
        phase: String,
        progressKnown: Boolean,
    ) {
        if (!supported(context)) return
        val succeeded = terminal && phase == "completed"
        val percent = if (succeeded) 100 else progress.coerceIn(0, 100)
        val percentText = if (progressKnown || succeeded) "$percent%" else "执行中"
        val terminalText = when (phase) {
            "completed" -> "已完成"
            "failed" -> "执行失败"
            "cancelled" -> "已取消"
            "interrupted" -> "已中断"
            else -> "已结束"
        }
        val statusText = when {
            terminal -> terminalText
            phase == "waiting_approval" -> "待确认"
            phase == "waiting_tool" -> "等待工具"
            phase == "queued" -> "排队中"
            phase == "starting" -> "启动中"
            else -> percentText
        }
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
                                    .put("title", statusText)
                                    .put("content", if (succeeded) "100%" else if (terminal) "已结束" else "执行中")
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
                                    .put("frontTitle", if (progressKnown || succeeded) "执行进度" else "任务状态")
                                    .put("title", statusText),
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
                .put("aodTitle", "$heading · $statusText")
                .put("ticker", "$heading · $statusText")
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
