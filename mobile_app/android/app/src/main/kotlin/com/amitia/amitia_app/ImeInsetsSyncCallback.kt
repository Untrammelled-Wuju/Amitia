package com.amitia.amitia_app

import android.graphics.Insets
import android.os.Build
import android.view.View
import android.view.WindowInsets
import android.view.WindowInsetsAnimation
import androidx.annotation.RequiresApi

@RequiresApi(Build.VERSION_CODES.R)
class ImeInsetsSyncCallback(private var view: View) {
    private val deferredInsetTypes = WindowInsets.Type.ime()
    private var lastWindowInsets: WindowInsets? = null
    private var animating = false
    private var needsSave = false

    private val animationCallback = object : WindowInsetsAnimation.Callback(DISPATCH_MODE_CONTINUE_ON_SUBTREE) {
        override fun onPrepare(animation: WindowInsetsAnimation) {
            needsSave = true
            if (animation.typeMask and deferredInsetTypes != 0) {
                animating = true
            }
        }

        override fun onProgress(
            insets: WindowInsets,
            runningAnimations: MutableList<WindowInsetsAnimation>,
        ): WindowInsets {
            if (!animating || needsSave) return insets
            if (runningAnimations.none { it.typeMask and deferredInsetTypes != 0 }) {
                return insets
            }
            view.onApplyWindowInsets(insets)
            return insets
        }

        override fun onEnd(animation: WindowInsetsAnimation) {
            if (!animating || animation.typeMask and deferredInsetTypes == 0) return
            animating = false
            lastWindowInsets?.let { view.dispatchApplyWindowInsets(it) }
        }
    }

    private val insetsListener = View.OnApplyWindowInsetsListener { source, insets ->
        view = source
        if (needsSave) {
            lastWindowInsets = insets
            needsSave = false
        }
        if (animating) {
            WindowInsets.CONSUMED
        } else {
            source.onApplyWindowInsets(insets)
        }
    }

    fun install() {
        view.setWindowInsetsAnimationCallback(animationCallback)
        view.setOnApplyWindowInsetsListener(insetsListener)
    }

    fun remove() {
        view.setWindowInsetsAnimationCallback(null)
        view.setOnApplyWindowInsetsListener(null)
    }
}
