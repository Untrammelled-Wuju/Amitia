package com.amitia.amitia_app.virtualdisplay.host

import android.annotation.SuppressLint
import android.os.SystemClock
import android.view.InputDevice
import android.view.InputEvent
import android.view.KeyEvent
import android.view.MotionEvent
import java.lang.reflect.InvocationTargetException
import java.lang.reflect.Method

@SuppressLint("PrivateApi", "BlockedPrivateApi")
internal class VirtualDisplayHostInputController {
    private val inputManager: Any
    private val injectInputEventMethod: Method
    private val setDisplayIdMethod: Method?

    init {
        val inputManagerClass = Class.forName("android.hardware.input.InputManager")
        val getInstance = inputManagerClass.getDeclaredMethod("getInstance")
        getInstance.isAccessible = true
        inputManager = getInstance.invoke(null)
            ?: throw IllegalStateException("Android InputManager unavailable")
        injectInputEventMethod = inputManagerClass.getDeclaredMethod(
            "injectInputEvent",
            InputEvent::class.java,
            Int::class.javaPrimitiveType,
        ).also { it.isAccessible = true }
        setDisplayIdMethod = try {
            InputEvent::class.java.getDeclaredMethod("setDisplayId", Int::class.javaPrimitiveType)
                .also { it.isAccessible = true }
        } catch (_: Throwable) {
            null
        }
    }

    fun tap(displayId: Int, x: Float, y: Float): Boolean {
        val now = SystemClock.uptimeMillis()
        val down = MotionEvent.obtain(
            now,
            now,
            MotionEvent.ACTION_DOWN,
            x,
            y,
            1f,
            1f,
            0,
            1f,
            1f,
            0,
            0,
        ).apply { source = InputDevice.SOURCE_TOUCHSCREEN }
        val up = MotionEvent.obtain(
            now,
            now + 60L,
            MotionEvent.ACTION_UP,
            x,
            y,
            1f,
            1f,
            0,
            1f,
            1f,
            0,
            0,
        ).apply { source = InputDevice.SOURCE_TOUCHSCREEN }
        return inject(displayId, down) and inject(displayId, up)
    }

    fun swipe(displayId: Int, x1: Float, y1: Float, x2: Float, y2: Float, durationMs: Long): Boolean {
        val start = SystemClock.uptimeMillis()
        val duration = durationMs.coerceIn(1L, 5000L)
        val steps = ((duration / 16L).coerceIn(2L, 120L)).toInt()
        val down = MotionEvent.obtain(
            start,
            start,
            MotionEvent.ACTION_DOWN,
            x1,
            y1,
            1f,
            1f,
            0,
            1f,
            1f,
            0,
            0,
        ).apply { source = InputDevice.SOURCE_TOUCHSCREEN }
        if (!inject(displayId, down)) return false
        var injected = true
        for (index in 1 until steps) {
            val fraction = index.toFloat() / steps.toFloat()
            val x = x1 + (x2 - x1) * fraction
            val y = y1 + (y2 - y1) * fraction
            val move = MotionEvent.obtain(
                start,
                start + (duration * fraction).toLong(),
                MotionEvent.ACTION_MOVE,
                x,
                y,
                1f,
                1f,
                0,
                1f,
                1f,
                0,
                0,
            ).apply { source = InputDevice.SOURCE_TOUCHSCREEN }
            injected = inject(displayId, move) && injected
        }
        val up = MotionEvent.obtain(
            start,
            start + duration,
            MotionEvent.ACTION_UP,
            x2,
            y2,
            1f,
            1f,
            0,
            1f,
            1f,
            0,
            0,
        ).apply { source = InputDevice.SOURCE_TOUCHSCREEN }
        return inject(displayId, up) && injected
    }

    fun key(displayId: Int, keyCode: Int, metaState: Int): Boolean {
        val now = SystemClock.uptimeMillis()
        val down = KeyEvent(now, now, KeyEvent.ACTION_DOWN, keyCode, 0, metaState).apply {
            source = InputDevice.SOURCE_KEYBOARD
        }
        val up = KeyEvent(now, now + 40L, KeyEvent.ACTION_UP, keyCode, 0, metaState).apply {
            source = InputDevice.SOURCE_KEYBOARD
        }
        return inject(displayId, down) and inject(displayId, up)
    }

    private fun inject(displayId: Int, event: InputEvent): Boolean {
        return try {
            if (setDisplayIdMethod != null && displayId != 0) {
                setDisplayIdMethod.invoke(event, displayId)
            }
            injectInputEventMethod.invoke(inputManager, event, 0) as? Boolean ?: false
        } catch (_: InvocationTargetException) {
            false
        } catch (_: Throwable) {
            false
        } finally {
            try {
                (event as? MotionEvent)?.recycle()
            } catch (_: Throwable) {
            }
        }
    }
}
