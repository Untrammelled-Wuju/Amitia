package com.amitia.amitia_app

import android.os.Build
import android.os.Bundle
import android.view.View
import android.view.ViewGroup
import com.amitia.amitia_app.nativeprovider.AndroidNativeBridgePlugin
import com.amitia.amitia_app.nativeprovider.AndroidNativeCompositionRoot
import com.amitia.amitia_app.runtime.bridge.RuntimeBridgePlugin
import com.amitia.amitia_app.realtime.RealtimeAudioPlugin
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.android.FlutterView
import io.flutter.embedding.engine.FlutterEngine
import java.lang.ref.WeakReference

class MainActivity : FlutterActivity() {
    companion object {
        @Volatile private var activeActivity: WeakReference<MainActivity>? = null

        fun currentActivity(): MainActivity? = activeActivity?.get()
    }

    private var imeInsetsSyncCallback: ImeInsetsSyncCallback? = null

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        AndroidNativeCompositionRoot.initialize(applicationContext)
    }

    override fun onResume() {
        super.onResume()
        activeActivity = WeakReference(this)
    }

    override fun onPause() {
        if (activeActivity?.get() === this) activeActivity = null
        super.onPause()
    }

    override fun onDestroy() {
        if (activeActivity?.get() === this) activeActivity = null
        super.onDestroy()
    }

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        flutterEngine.plugins.add(RuntimeBridgePlugin())
        flutterEngine.plugins.add(AndroidNativeBridgePlugin())
        flutterEngine.plugins.add(RealtimeAudioPlugin())
    }

    override fun onPostResume() {
        super.onPostResume()
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.R) return
        window.decorView.post {
            val flutterView = findFlutterView(window.decorView) ?: return@post
            imeInsetsSyncCallback?.remove()
            imeInsetsSyncCallback = ImeInsetsSyncCallback(flutterView).also { it.install() }
        }
    }

    private fun findFlutterView(view: View): FlutterView? {
        if (view is FlutterView) return view
        if (view !is ViewGroup) return null
        for (index in 0 until view.childCount) {
            findFlutterView(view.getChildAt(index))?.let { return it }
        }
        return null
    }
}
