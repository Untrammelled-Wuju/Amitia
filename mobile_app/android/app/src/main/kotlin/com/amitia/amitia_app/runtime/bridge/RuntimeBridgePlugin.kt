package com.amitia.amitia_app.runtime.bridge

import com.amitia.amitia_app.runtime.AndroidRuntimeModule
import com.amitia.amitia_app.runtime.api.RuntimeController
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestStore
import android.content.Context
import io.flutter.embedding.engine.plugins.FlutterPlugin
import io.flutter.plugin.common.EventChannel
import io.flutter.plugin.common.MethodChannel

class RuntimeBridgePlugin : FlutterPlugin {

    private var methodChannel: MethodChannel? = null
    private var eventChannel: EventChannel? = null
    private var methodHandler: RuntimeBridgeHandler? = null
    private var streamHandler: RuntimeBridgeStreamHandler? = null

    override fun onAttachedToEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        val context = binding.applicationContext
        val module = AndroidRuntimeModule.create(context)
        val controller = module.controller
        val backendConnectionProvider = module.backendConnectionProvider

        val methodChannel = MethodChannel(binding.binaryMessenger, RuntimeBridgeContract.METHOD_CHANNEL)
        val methodHandler = RuntimeBridgeHandler(
            controller = controller,
            backendConnectionProvider = backendConnectionProvider,
            manifestStore = null,
        )
        methodChannel.setMethodCallHandler(methodHandler)
        this.methodChannel = methodChannel
        this.methodHandler = methodHandler

        val eventChannel = EventChannel(binding.binaryMessenger, RuntimeBridgeContract.EVENT_CHANNEL)
        val streamHandler = RuntimeBridgeStreamHandler(
            controller = controller,
            manifestStore = null,
        )
        eventChannel.setStreamHandler(streamHandler)
        this.eventChannel = eventChannel
        this.streamHandler = streamHandler
    }

    override fun onDetachedFromEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        methodChannel?.setMethodCallHandler(null)
        eventChannel?.setStreamHandler(null)
        streamHandler?.detach()
        methodChannel = null
        eventChannel = null
        methodHandler = null
        streamHandler = null
    }
}
