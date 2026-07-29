package com.amitia.runtime.extension

import com.amitia.core.database.dao.ExtensionInstallationDao
import com.amitia.runtime.BuildConfig
import com.amitia.runtime.extension.security.PublisherTrustStore
import com.amitia.runtime.extension.security.RemotePublisherTrustStore
import com.amitia.runtime.extension.security.RevocationList
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
    fun provideRevocationList(): RevocationList = RevocationList()

    @Provides
    @Singleton
    fun providePublisherTrustStore(apiClient: ExtensionApiClient): PublisherTrustStore =
        RemotePublisherTrustStore(apiClient)

    @Provides
    @Singleton
    fun provideAmitiaxPackageLoader(
        trustStore: PublisherTrustStore,
        revocationList: RevocationList
    ): AmitiaxPackageLoader =
        AmitiaxPackageLoader.forProduction(
            trustStore = trustStore,
            revocationList = revocationList,
            isDebug = BuildConfig.DEBUG
        )

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
        trustStore: PublisherTrustStore,
        json: Json
    ): ExtensionHost = ExtensionHostImpl(
        packageLoader,
        toolExecutor,
        apiClient,
        installationDao,
        permissionChecker,
        trustStore,
        json
    )
}
