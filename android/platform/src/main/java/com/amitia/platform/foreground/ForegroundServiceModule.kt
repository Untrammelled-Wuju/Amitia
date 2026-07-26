package com.amitia.platform.foreground

import dagger.Binds
import dagger.Module
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton

@Module
@InstallIn(SingletonComponent::class)
abstract class ForegroundServiceModule {

    @Binds
    @Singleton
    abstract fun bindForegroundServiceManager(impl: ForegroundServiceManagerImpl): ForegroundServiceManager
}
