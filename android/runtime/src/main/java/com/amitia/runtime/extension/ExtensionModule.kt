package com.amitia.runtime.extension

import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton

@Module
@InstallIn(SingletonComponent::class)
object ExtensionModule {

    @Provides
    @Singleton
    fun provideAmitiaxPackageLoader(): AmitiaxPackageLoader = AmitiaxPackageLoader()

    @Provides
    @Singleton
    fun provideToolExecutor(): ToolExecutor = LocalToolExecutor()

    @Provides
    @Singleton
    fun provideExtensionHost(
        packageLoader: AmitiaxPackageLoader,
        toolExecutor: ToolExecutor
    ): ExtensionHost = ExtensionHostImpl(packageLoader, toolExecutor)
}
