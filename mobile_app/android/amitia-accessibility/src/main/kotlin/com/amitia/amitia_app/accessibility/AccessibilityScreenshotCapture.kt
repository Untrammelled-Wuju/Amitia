package com.amitia.amitia_app.accessibility

import android.accessibilityservice.AccessibilityService
import android.accessibilityservice.AccessibilityServiceInfo
import android.graphics.Bitmap
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.util.Base64
import java.io.ByteArrayOutputStream
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit

object AccessibilityScreenshotCapture {

    data class Result(
        val success: Boolean,
        val imageBase64: String = "",
        val mimeType: String = "",
        val width: Int = 0,
        val height: Int = 0,
        val message: String = "",
    )

    fun capture(
        service: AccessibilityService,
        displayId: Int,
        timeoutMs: Long = 10000L,
    ): Result {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.R) {
            return Result(false, message = "accessibility screenshot requires Android 11 or newer")
        }
        val capabilities = service.serviceInfo?.capabilities ?: 0
        if (capabilities and AccessibilityServiceInfo.CAPABILITY_CAN_TAKE_SCREENSHOT == 0) {
            return Result(false, message = "accessibility screenshot capability is not available")
        }
        val latch = CountDownLatch(1)
        val resultRef = java.util.concurrent.atomic.AtomicReference(
            Result(false, message = "screenshot callback was not completed"),
        )
        val handler = Handler(Looper.getMainLooper())
        handler.post {
            try {
                service.takeScreenshot(
                    displayId,
                    { runnable -> handler.post(runnable) },
                    object : AccessibilityService.TakeScreenshotCallback {
                        override fun onSuccess(screenshot: AccessibilityService.ScreenshotResult) {
                            try {
                                val bitmap = Bitmap.wrapHardwareBuffer(
                                    screenshot.hardwareBuffer,
                                    screenshot.colorSpace,
                                )
                                if (bitmap == null) {
                                    resultRef.set(Result(false, message = "failed to wrap screenshot buffer"))
                                    return
                                }
                                val output = ByteArrayOutputStream()
                                bitmap.compress(Bitmap.CompressFormat.JPEG, 85, output)
                                val bytes = output.toByteArray()
                                resultRef.set(
                                    Result(
                                        success = true,
                                        imageBase64 = Base64.encodeToString(bytes, Base64.NO_WRAP),
                                        mimeType = "image/jpeg",
                                        width = bitmap.width,
                                        height = bitmap.height,
                                    ),
                                )
                            } catch (error: Throwable) {
                                resultRef.set(
                                    Result(false, message = error.message ?: error.javaClass.simpleName),
                                )
                            } finally {
                                screenshot.hardwareBuffer.close()
                                latch.countDown()
                            }
                        }

                        override fun onFailure(errorCode: Int) {
                            resultRef.set(Result(false, message = "screenshot failed with code $errorCode"))
                            latch.countDown()
                        }
                    },
                )
            } catch (error: Throwable) {
                resultRef.set(Result(false, message = error.message ?: error.javaClass.simpleName))
                latch.countDown()
            }
        }
        return try {
            if (latch.await(timeoutMs.coerceAtLeast(1L), TimeUnit.MILLISECONDS)) {
                resultRef.get()
            } else {
                Result(false, message = "screenshot timed out")
            }
        } catch (_: InterruptedException) {
            Thread.currentThread().interrupt()
            Result(false, message = "screenshot interrupted")
        }
    }
}
