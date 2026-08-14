package com.amitia.amitia_app

import android.os.Build
import android.os.Bundle
import android.view.View
import android.view.ViewGroup
import com.amitia.amitia_app.nativeprovider.AndroidNativeBridgePlugin
import com.amitia.amitia_app.nativeprovider.AndroidNativeCompositionRoot
import com.amitia.amitia_app.runtime.bridge.RuntimeBridgePlugin
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.android.FlutterView
import io.flutter.embedding.engine.FlutterEngine

class MainActivity : FlutterActivity() {
    private var imeInsetsSyncCallback: ImeInsetsSyncCallback? = null

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        AndroidNativeCompositionRoot.initialize(applicationContext)
    }

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        flutterEngine.plugins.add(RuntimeBridgePlugin())
        flutterEngine.plugins.add(AndroidNativeBridgePlugin())
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
