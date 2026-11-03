package com.amitia.amitia_app.virtualdisplay.preview

import android.app.Activity
import android.graphics.Color
import android.os.Bundle
import android.util.Base64
import android.util.Log
import android.view.Gravity
import android.widget.TextView
import com.amitia.amitia_app.virtualdisplay.host.VirtualDisplayHostManager
import com.amitia.amitia_app.virtualdisplay.host.VirtualDisplayPermissionStore
import kotlinx.coroutines.delay
import kotlinx.coroutines.runBlocking

class VirtualDisplayPreviewDebugActivity : Activity() {
    private lateinit var statusView: TextView

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        statusView = TextView(this).apply {
            gravity = Gravity.CENTER
            setTextColor(Color.WHITE)
            setBackgroundColor(Color.BLACK)
            textSize = 16f
            text = "准备虚拟屏测试"
        }
        setContentView(statusView)
        val context = applicationContext
        Thread {
            runBlocking {
                try {
                    setStatus("启用虚拟屏")
                    VirtualDisplayPermissionStore.setEnabled(context, true)
                    val manager = VirtualDisplayHostManager(context)
                    Log.i("AmitiaPreviewE2E", "ensureStarted")
                    setStatus("连接虚拟屏主机")
                    manager.ensureStarted()
                    Log.i("AmitiaPreviewE2E", "ensureStarted complete")
                    setStatus("创建虚拟屏")
                    val created = manager.execute(
                        "virtual_display.create",
                        mapOf(
                            "name" to "amitia_preview_debug",
                            "width" to 720,
                            "height" to 1280,
                            "densityDpi" to 320,
                        ),
                    )
                    val displayId = (created["displayId"] as? Number)?.toInt()
                        ?: throw IllegalStateException("missing displayId: $created")
                    Log.i("AmitiaPreviewE2E", "create complete displayId=$displayId")
                    setStatus("启动计算器 display=$displayId")
                    manager.execute(
                        "virtual_display.launch",
                        mapOf(
                            "displayId" to displayId,
                            "packageName" to "com.miui.calculator",
                        ),
                    )
                    Log.i("AmitiaPreviewE2E", "launch complete")
                    setStatus("等待虚拟屏画面")
                    delay(2000L)
                    setStatus("截取虚拟屏画面")
                    val capture = manager.execute(
                        "virtual_display.capture",
                        mapOf(
                            "displayId" to displayId,
                            "format" to "jpeg",
                            "quality" to 70,
                            "maxWidth" to 540,
                            "maxHeight" to 960,
                        ),
                    )
                    val encoded = capture["dataBase64"] as? String
                        ?: throw IllegalStateException("capture missing dataBase64")
                    val bytes = Base64.decode(encoded, Base64.NO_WRAP)
                    if (bytes.size < 1024) {
                        throw IllegalStateException("capture frame too small: ${bytes.size}")
                    }
                    Log.i(
                        "AmitiaPreviewE2E",
                        "displayId=$displayId captureBytes=${bytes.size}",
                    )
                    setStatus("启动悬浮窗")
                    VirtualDisplayPreviewService.setEnabled(context, true)
                    VirtualDisplayPreviewService.onVirtualDisplayOperation(context, displayId)
                    Log.i("AmitiaPreviewE2E", "preview requested")
                    delay(2000L)
                    setStatus("虚拟屏测试已启动 display=$displayId frame=${bytes.size}")
                } catch (error: Throwable) {
                    setStatus("测试失败\n${error.javaClass.name}: ${error.message}")
                    Log.e(
                        "AmitiaPreviewE2E",
                        "${error.javaClass.name}: ${error.message}\n${error.stackTraceToString()}",
                    )
                }
            }
        }.start()
    }

    private fun setStatus(value: String) {
        runOnUiThread {
            statusView.text = value
        }
    }
}
