package com.amitia.amitia_app

import android.os.Build
import android.os.Bundle
import android.view.View
import android.view.ViewGroup
import com.amitia.amitia_app.runtime.bridge.RuntimeBridgeContract
import com.amitia.amitia_app.runtime.bridge.RuntimeBridgeHandler
import com.amitia.amitia_app.runtime.bridge.RuntimeBridgeStreamHandler
import com.amitia.amitia_app.runtime.AndroidRuntimeModule
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.android.FlutterView
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.EventChannel
import io.flutter.plugin.common.MethodChannel

class MainActivity : FlutterActivity() {
    private var imeInsetsSyncCallback: ImeInsetsSyncCallback? = null
    private var methodChannel: MethodChannel? = null
    private var eventChannel: EventChannel? = null
    private var bridgeHandler: RuntimeBridgeHandler? = null
    private var streamHandler: RuntimeBridgeStreamHandler? = null

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
    }

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        registerRuntimeBridge(flutterEngine)
    }

    private fun registerRuntimeBridge(flutterEngine: FlutterEngine) {
        val module = AndroidRuntimeModule.create(applicationContext)
        val controller = module.controller

        val methodChannel = MethodChannel(
            flutterEngine.dartExecutor.binaryMessenger,
            RuntimeBridgeContract.METHOD_CHANNEL
        )
        val handler = RuntimeBridgeHandler(
            controller = controller,
            manifestStore = null,
        )
        methodChannel.setMethodCallHandler(handler)
        this.methodChannel = methodChannel
        this.bridgeHandler = handler

        val eventChannel = EventChannel(
            flutterEngine.dartExecutor.binaryMessenger,
            RuntimeBridgeContract.EVENT_CHANNEL
        )
        val streamHandler = RuntimeBridgeStreamHandler(
            controller = controller,
            manifestStore = null,
        )
        eventChannel.setStreamHandler(streamHandler)
        this.eventChannel = eventChannel
        this.streamHandler = streamHandler
    }

    override fun cleanUpFlutterEngine(flutterEngine: FlutterEngine) {
        super.cleanUpFlutterEngine(flutterEngine)
        methodChannel?.setMethodCallHandler(null)
        eventChannel?.setStreamHandler(null)
        streamHandler?.detach()
        methodChannel = null
        eventChannel = null
        bridgeHandler = null
        streamHandler = null
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
