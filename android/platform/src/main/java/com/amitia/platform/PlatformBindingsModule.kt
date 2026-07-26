package com.amitia.platform.audio

import com.amitia.platform.files.FilePicker
import com.amitia.platform.files.FilePickerImpl
import dagger.Binds
import dagger.Module
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton

@Module
@InstallIn(SingletonComponent::class)
abstract class PlatformBindingsModule {

    @Binds
    @Singleton
    abstract fun bindFilePicker(impl: FilePickerImpl): FilePicker

    @Binds
    @Singleton
    abstract fun bindAudioPlayer(impl: AudioPlayerImpl): AudioPlayer

    @Binds
    @Singleton
    abstract fun bindAudioRecorder(impl: AudioRecorderImpl): AudioRecorder
}
