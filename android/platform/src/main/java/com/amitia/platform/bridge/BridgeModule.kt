package com.amitia.platform.bridge

import com.amitia.platform.bridge.provider.AppDirProvider
import com.amitia.platform.bridge.provider.AudioPlayProvider
import com.amitia.platform.bridge.provider.BatteryStateProvider
import com.amitia.platform.bridge.provider.CameraProvider
import com.amitia.platform.bridge.provider.ClipboardProvider
import com.amitia.platform.bridge.provider.FilePickProvider
import com.amitia.platform.bridge.provider.ForegroundStateProvider
import com.amitia.platform.bridge.provider.ImagePickProvider
import com.amitia.platform.bridge.provider.MicRecordProvider
import com.amitia.platform.bridge.provider.NetworkStateProvider
import com.amitia.platform.bridge.provider.NotificationProvider
import com.amitia.platform.bridge.provider.ShareProvider
import com.amitia.platform.bridge.provider.SystemThemeProvider
import dagger.Binds
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import dagger.multibindings.IntoMap
import javax.inject.Singleton

@Module
@InstallIn(SingletonComponent::class)
abstract class BridgeModule {

    @Binds
    @Singleton
    abstract fun bindActivityResultBridge(impl: ActivityResultBridgeImpl): ActivityResultBridge
}

@Module
@InstallIn(SingletonComponent::class)
object BridgeProvidersModule {

    @Provides
    @IntoMap
    @ActionKey(NativeCapabilityBridge.Actions.FILE_PICK)
    @Singleton
    fun provideFilePickProvider(impl: FilePickProvider): CapabilityProvider = impl

    @Provides
    @IntoMap
    @ActionKey(NativeCapabilityBridge.Actions.IMAGE_PICK)
    @Singleton
    fun provideImagePickProvider(impl: ImagePickProvider): CapabilityProvider = impl

    @Provides
    @IntoMap
    @ActionKey(NativeCapabilityBridge.Actions.CAMERA)
    @Singleton
    fun provideCameraProvider(impl: CameraProvider): CapabilityProvider = impl

    @Provides
    @IntoMap
    @ActionKey(NativeCapabilityBridge.Actions.MIC_RECORD)
    @Singleton
    fun provideMicRecordProvider(impl: MicRecordProvider): CapabilityProvider = impl

    @Provides
    @IntoMap
    @ActionKey(NativeCapabilityBridge.Actions.AUDIO_PLAY)
    @Singleton
    fun provideAudioPlayProvider(impl: AudioPlayProvider): CapabilityProvider = impl

    @Provides
    @IntoMap
    @ActionKey(NativeCapabilityBridge.Actions.NOTIFICATION)
    @Singleton
    fun provideNotificationProvider(impl: NotificationProvider): CapabilityProvider = impl

    @Provides
    @IntoMap
    @ActionKey(NativeCapabilityBridge.Actions.CLIPBOARD_READ)
    @Singleton
    fun provideClipboardReadProvider(impl: ClipboardProvider): CapabilityProvider = impl

    @Provides
    @IntoMap
    @ActionKey(NativeCapabilityBridge.Actions.CLIPBOARD_WRITE)
    @Singleton
    fun provideClipboardWriteProvider(impl: ClipboardProvider): CapabilityProvider = impl

    @Provides
    @IntoMap
    @ActionKey(NativeCapabilityBridge.Actions.SHARE)
    @Singleton
    fun provideShareProvider(impl: ShareProvider): CapabilityProvider = impl

    @Provides
    @IntoMap
    @ActionKey(NativeCapabilityBridge.Actions.APP_DIR)
    @Singleton
    fun provideAppDirProvider(impl: AppDirProvider): CapabilityProvider = impl

    @Provides
    @IntoMap
    @ActionKey(NativeCapabilityBridge.Actions.SYSTEM_THEME)
    @Singleton
    fun provideSystemThemeProvider(impl: SystemThemeProvider): CapabilityProvider = impl

    @Provides
    @IntoMap
    @ActionKey(NativeCapabilityBridge.Actions.NETWORK_STATE)
    @Singleton
    fun provideNetworkStateProvider(impl: NetworkStateProvider): CapabilityProvider = impl

    @Provides
    @IntoMap
    @ActionKey(NativeCapabilityBridge.Actions.BATTERY_STATE)
    @Singleton
    fun provideBatteryStateProvider(impl: BatteryStateProvider): CapabilityProvider = impl

    @Provides
    @IntoMap
    @ActionKey(NativeCapabilityBridge.Actions.FOREGROUND_STATE)
    @Singleton
    fun provideForegroundStateProvider(impl: ForegroundStateProvider): CapabilityProvider = impl
}
