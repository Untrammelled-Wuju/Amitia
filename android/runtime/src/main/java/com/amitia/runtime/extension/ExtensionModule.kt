package com.amitia.runtime.extension

import com.amitia.core.database.dao.ExtensionInstallationDao
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton
import kotlinx.serialization.json.Json

@Module
@InstallIn(SingletonComponent::class)
object ExtensionModule {

    @Provides
    @Singleton
    fun provideBaseUrlProvider(provider: RuntimeBaseUrlProvider): BaseUrlProvider = provider

    @Provides
    @Singleton
    fun provideAmitiaxPackageLoader(): AmitiaxPackageLoader = AmitiaxPackageLoader()

    @Provides
    @Singleton
    fun provideToolExecutor(remoteToolExecutor: RemoteToolExecutor): ToolExecutor =
        remoteToolExecutor

    @Provides
    @Singleton
    fun provideExtensionHost(
        packageLoader: AmitiaxPackageLoader,
        toolExecutor: ToolExecutor,
        apiClient: ExtensionApiClient,
        installationDao: ExtensionInstallationDao,
        permissionChecker: ExtensionPermissionChecker,
        json: Json
    ): ExtensionHost = ExtensionHostImpl(
        packageLoader,
        toolExecutor,
        apiClient,
        installationDao,
        permissionChecker,
        json
    )
}
