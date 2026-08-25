package com.amitia.amitia_app.runtime.bridge

import com.amitia.amitia_app.runtime.AndroidRuntimeModule
import com.amitia.amitia_app.runtime.api.RuntimeController
import com.amitia.amitia_app.runtime.internal.DefaultRuntimeModule
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestStore
import com.amitia.amitia_app.runtime.packagetrusted.RuntimePackageSource
import com.amitia.amitia_app.runtime.bridge.RuntimeBridgeContract
import android.content.Context
import io.flutter.embedding.engine.plugins.FlutterPlugin
import io.flutter.plugin.common.EventChannel
import io.flutter.plugin.common.MethodChannel
import io.flutter.plugin.common.StandardMethodCodec

class RuntimeBridgePlugin : FlutterPlugin {

    private var methodChannel: MethodChannel? = null
    private var eventChannel: EventChannel? = null
    private var methodHandler: RuntimeBridgeHandler? = null
    private var streamHandler: RuntimeBridgeStreamHandler? = null

    override fun onAttachedToEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        val context = binding.applicationContext
        val module = AndroidRuntimeModule.create(context)
        val controller = module.controller
        val backendConnectionProvider = (module as DefaultRuntimeModule).backendConnectionProvider
        val manifestStore = (module as DefaultRuntimeModule).manifestStore
        val runtimePackageSource = AndroidRuntimeModule.runtimePackageSource
            ?: error("RuntimePackageSource is not initialized")

        // Runtime install/verify/start perform filesystem and integrity work. Run
        // method handlers on a serial background task queue so plugin attachment
        // and Flutter's Android platform thread cannot be blocked into an ANR.
        val runtimeTaskQueue = binding.binaryMessenger.makeBackgroundTaskQueue()
        val methodChannel = MethodChannel(
            binding.binaryMessenger,
            RuntimeBridgeContract.METHOD_CHANNEL,
            StandardMethodCodec.INSTANCE,
            runtimeTaskQueue,
        )
        val methodHandler = RuntimeBridgeHandler(
            controller = controller,
            backendConnectionProvider = backendConnectionProvider,
            manifestStore = manifestStore,
            runtimePackageSource = runtimePackageSource,
        )
        methodChannel.setMethodCallHandler(methodHandler)
        this.methodChannel = methodChannel
        this.methodHandler = methodHandler

        val eventChannel = EventChannel(binding.binaryMessenger, RuntimeBridgeContract.EVENT_CHANNEL)
        val streamHandler = RuntimeBridgeStreamHandler(
            controller = controller,
            manifestStore = manifestStore,
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
