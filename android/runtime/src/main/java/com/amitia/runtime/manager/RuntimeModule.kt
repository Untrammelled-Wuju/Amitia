package com.amitia.runtime.manager

import com.amitia.runtime.api.RuntimeFacade
import com.amitia.runtime.bootstrap.BootstrapSequence
import com.amitia.runtime.bootstrap.BootstrapSequenceImpl
import com.amitia.runtime.bootstrap.ShutdownSequence
import com.amitia.runtime.bootstrap.ShutdownSequenceImpl
import com.amitia.runtime.bridge.RuntimeBridge
import com.amitia.runtime.bridge.RuntimeBridgeStub
import com.amitia.runtime.bridge.RuntimeFacadeImpl
import com.amitia.runtime.health.HealthChecker
import com.amitia.runtime.health.HealthCheckerImpl
import com.amitia.runtime.linux.LinuxRootfsManager
import com.amitia.runtime.linux.LinuxRootfsManagerImpl
import com.amitia.runtime.linux.ProotBinaryManager
import com.amitia.runtime.linux.ProotBinaryManagerImpl
import com.amitia.runtime.process.LinuxProcessManager
import com.amitia.runtime.process.LinuxProcessManagerImpl
import com.amitia.runtime.process.ProotCommandWrapper
import com.amitia.runtime.process.ProotCommandWrapperImpl
import dagger.Binds
import dagger.Module
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton

@Module
@InstallIn(SingletonComponent::class)
abstract class RuntimeModule {

    @Binds
    @Singleton
    abstract fun bindLinuxRootfsManager(impl: LinuxRootfsManagerImpl): LinuxRootfsManager

    @Binds
    @Singleton
    abstract fun bindLinuxProcessManager(impl: LinuxProcessManagerImpl): LinuxProcessManager

    @Binds
    @Singleton
    abstract fun bindHealthChecker(impl: HealthCheckerImpl): HealthChecker

    @Binds
    @Singleton
    abstract fun bindBootstrapSequence(impl: BootstrapSequenceImpl): BootstrapSequence

    @Binds
    @Singleton
    abstract fun bindShutdownSequence(impl: ShutdownSequenceImpl): ShutdownSequence

    @Binds
    @Singleton
    abstract fun bindRuntimeFacade(impl: RuntimeFacadeImpl): RuntimeFacade

    @Binds
    @Singleton
    abstract fun bindRuntimeManager(impl: RuntimeManagerImpl): RuntimeManager

    @Binds
    @Singleton
    abstract fun bindRuntimeBridge(impl: RuntimeBridgeStub): RuntimeBridge

    @Binds
    @Singleton
    abstract fun bindProotBinaryManager(impl: ProotBinaryManagerImpl): ProotBinaryManager

    @Binds
    @Singleton
    abstract fun bindProotCommandWrapper(impl: ProotCommandWrapperImpl): ProotCommandWrapper
}
