package com.amitia.runtime.manager

import app.cash.turbine.test
import com.amitia.runtime.api.RuntimeEvent
import com.amitia.runtime.api.RuntimeServices
import com.amitia.runtime.api.RuntimeState
import com.amitia.runtime.api.ServiceState
import com.google.common.truth.Truth.assertThat
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.runTest
import org.junit.Test

class RuntimeStateMachineTest {

    private val machine = RuntimeStateMachine()

    private fun healthyServices(): RuntimeServices = RuntimeServices(
        surrealDb = ServiceState.Healthy(18000),
        qdrant = ServiceState.Healthy(18001),
        backend = ServiceState.Healthy(18899)
    )

    @Test
    fun initial_state_is_NotInstalled() {
        assertThat(machine.current).isEqualTo(RuntimeState.NotInstalled)
        assertThat(machine.current.phase).isEqualTo(RuntimeState.Phase.IDLE)
    }

    @Test
    fun valid_transition_IDLE_to_INSTALLING_succeeds() = runTest {
        val result = machine.transition(RuntimeState.Installing(progressValue = 0.1f, message = "extracting"))

        assertThat(result.isSuccess).isTrue()
        assertThat(machine.current.phase).isEqualTo(RuntimeState.Phase.INSTALLING)
    }

    @Test
    fun valid_transition_IDLE_to_INSTALLED_succeeds() = runTest {
        val result = machine.transition(RuntimeState.Installed)

        assertThat(result.isSuccess).isTrue()
        assertThat(machine.current).isEqualTo(RuntimeState.Installed)
    }

    @Test
    fun valid_transition_IDLE_to_STARTING_succeeds() = runTest {
        val result = machine.transition(RuntimeState.Starting(stage = "init", progressValue = 0.1f))

        assertThat(result.isSuccess).isTrue()
        assertThat(machine.current.phase).isEqualTo(RuntimeState.Phase.STARTING)
    }

    @Test
    fun valid_full_path_Installed_to_Starting_to_Running() = runTest {
        machine.transition(RuntimeState.Installed)
        machine.transition(RuntimeState.Starting(stage = "backend", progressValue = 0.5f))

        val result = machine.transition(RuntimeState.Running(uptimeMs = 0L, services = healthyServices()))

        assertThat(result.isSuccess).isTrue()
        assertThat(machine.current.phase).isEqualTo(RuntimeState.Phase.RUNNING)
    }

    @Test
    fun invalid_transition_IDLE_to_RUNNING_fails() = runTest {
        val result = machine.transition(RuntimeState.Running(uptimeMs = 0L, services = healthyServices()))

        assertThat(result.isFailure).isTrue()
        val error = result.exceptionOrNull()
        assertThat(error).isInstanceOf(IllegalStateException::class.java)
        assertThat(error!!.message).contains("非法状态转换")
        assertThat(machine.current).isEqualTo(RuntimeState.NotInstalled)
    }

    @Test
    fun invalid_transition_RUNNING_to_INSTALLING_fails() = runTest {
        machine.transition(RuntimeState.Installed)
        machine.transition(RuntimeState.Starting(stage = "backend", progressValue = 0.5f))
        machine.transition(RuntimeState.Running(uptimeMs = 0L, services = healthyServices()))

        val result = machine.transition(RuntimeState.Installing(progressValue = 0.5f, message = "should not allowed"))

        assertThat(result.isFailure).isTrue()
        assertThat(machine.current.phase).isEqualTo(RuntimeState.Phase.RUNNING)
    }

    @Test
    fun valid_path_RUNNING_to_STOPPING_to_STOPPED() = runTest {
        machine.transition(RuntimeState.Installed)
        machine.transition(RuntimeState.Starting(stage = "backend", progressValue = 0.5f))
        machine.transition(RuntimeState.Running(uptimeMs = 0L, services = healthyServices()))

        val stopResult = machine.transition(RuntimeState.Stopping(stage = "backend"))
        assertThat(stopResult.isSuccess).isTrue()

        val stoppedResult = machine.transition(RuntimeState.Stopped)
        assertThat(stoppedResult.isSuccess).isTrue()
        assertThat(machine.current).isEqualTo(RuntimeState.Stopped)
    }

    @Test
    fun valid_path_FAILED_to_STARTING_recovery() = runTest {
        machine.transition(RuntimeState.Installed)
        machine.transition(RuntimeState.Starting(stage = "backend", progressValue = 0.5f))
        machine.transition(RuntimeState.Failed(errorMessage = "boom", retryable = true, requiresUserAction = false))

        val result = machine.transition(RuntimeState.Starting(stage = "recover", progressValue = 0.2f))

        assertThat(result.isSuccess).isTrue()
        assertThat(machine.current.phase).isEqualTo(RuntimeState.Phase.STARTING)
    }

    @Test
    fun valid_path_RUNNING_to_DEGRADED_to_RUNNING_recovered() = runTest {
        machine.transition(RuntimeState.Installed)
        machine.transition(RuntimeState.Starting(stage = "backend", progressValue = 0.5f))
        machine.transition(RuntimeState.Running(uptimeMs = 0L, services = healthyServices()))
        machine.transition(
            RuntimeState.Degraded(
                reason = "surrealdb",
                services = RuntimeServices(
                    surrealDb = ServiceState.Unhealthy("timeout"),
                    qdrant = ServiceState.Healthy(18001),
                    backend = ServiceState.Healthy(18899)
                )
            )
        )

        val result = machine.transition(
            RuntimeState.Running(uptimeMs = 100L, services = healthyServices())
        )

        assertThat(result.isSuccess).isTrue()
        assertThat(machine.current.phase).isEqualTo(RuntimeState.Phase.RUNNING)
    }

    @Test
    fun valid_path_INSTALLED_to_UPDATING_to_INSTALLED() = runTest {
        machine.transition(RuntimeState.Installed)

        val updating = machine.transition(RuntimeState.Updating(progressValue = 0.5f, message = "patching"))
        assertThat(updating.isSuccess).isTrue()

        val installed = machine.transition(RuntimeState.Installed)
        assertThat(installed.isSuccess).isTrue()
        assertThat(machine.current).isEqualTo(RuntimeState.Installed)
    }

    @Test
    fun transition_emits_StateChanged_event() = runTest {
        machine.events.test {
            machine.transition(RuntimeState.Installed)

            val event = awaitItem()
            assertThat(event).isInstanceOf(RuntimeEvent.StateChanged::class.java)
            val stateChanged = event as RuntimeEvent.StateChanged
            assertThat(stateChanged.from).isEqualTo(RuntimeState.NotInstalled)
            assertThat(stateChanged.to).isEqualTo(RuntimeState.Installed)

            val progressEvent = awaitItem()
            assertThat(progressEvent).isInstanceOf(RuntimeEvent.ProgressUpdated::class.java)
            cancelAndIgnoreRemainingEvents()
        }
    }

    @Test
    fun transition_emits_ProgressUpdated_event_after_state_change() = runTest {
        machine.events.test {
            machine.transition(RuntimeState.Installed)

            awaitItem()
            val progressEvent = awaitItem()
            assertThat(progressEvent).isInstanceOf(RuntimeEvent.ProgressUpdated::class.java)
        }
    }

    @Test
    fun invalid_transition_emits_ErrorOccurred_event() = runTest {
        machine.events.test {
            machine.transition(RuntimeState.Running(uptimeMs = 0L, services = healthyServices()))

            val event = awaitItem()
            assertThat(event).isInstanceOf(RuntimeEvent.ErrorOccurred::class.java)
            val err = event as RuntimeEvent.ErrorOccurred
            assertThat(err.retryable).isFalse()
            assertThat(err.requiresUserAction).isTrue()
        }
    }

    @Test
    fun emitProgress_emits_ProgressUpdated_event() = runTest {
        machine.events.test {
            machine.emitProgress(RuntimeState.Installed)

            val event = awaitItem()
            assertThat(event).isInstanceOf(RuntimeEvent.ProgressUpdated::class.java)
            assertThat((event as RuntimeEvent.ProgressUpdated).stage.stage).isEqualTo("Installed")
        }
    }

    @Test
    fun emitLog_emits_LogEmitted_event() = runTest {
        machine.events.test {
            machine.emitLog(
                RuntimeEvent.LogEmitted.Level.INFO,
                "TestTag",
                "test message"
            )

            val event = awaitItem()
            assertThat(event).isInstanceOf(RuntimeEvent.LogEmitted::class.java)
            val log = event as RuntimeEvent.LogEmitted
            assertThat(log.tag).isEqualTo("TestTag")
            assertThat(log.message).isEqualTo("test message")
            assertThat(log.level).isEqualTo(RuntimeEvent.LogEmitted.Level.INFO)
        }
    }

    @Test
    fun emitServiceHealth_emits_ServiceHealthChanged_event() = runTest {
        machine.events.test {
            machine.emitServiceHealth("qdrant", ServiceState.Healthy(18001))

            val event = awaitItem()
            assertThat(event).isInstanceOf(RuntimeEvent.ServiceHealthChanged::class.java)
            val health = event as RuntimeEvent.ServiceHealthChanged
            assertThat(health.serviceName).isEqualTo("qdrant")
            assertThat(health.state).isInstanceOf(ServiceState.Healthy::class.java)

            val healthyEvent = awaitItem()
            assertThat(healthyEvent).isInstanceOf(RuntimeEvent.ServiceHealthy::class.java)
            val healthy = healthyEvent as RuntimeEvent.ServiceHealthy
            assertThat(healthy.serviceName).isEqualTo("qdrant")
            assertThat(healthy.port).isEqualTo(18001)
        }
    }

    @Test
    fun emitError_emits_ErrorOccurred_with_supplied_fields() = runTest {
        machine.events.test {
            val cause = IllegalStateException("root cause")
            machine.emitError(
                error = "backend crashed",
                retryable = true,
                requiresUserAction = false,
                cause = cause
            )

            val event = awaitItem()
            assertThat(event).isInstanceOf(RuntimeEvent.ErrorOccurred::class.java)
            val err = event as RuntimeEvent.ErrorOccurred
            assertThat(err.error).isEqualTo("backend crashed")
            assertThat(err.retryable).isTrue()
            assertThat(err.requiresUserAction).isFalse()
            assertThat(err.cause).isSameInstanceAs(cause)
        }
    }

    @Test
    fun observe_returns_same_flow_as_state() = runTest {
        machine.transition(RuntimeState.Installed)

        val stateFromObserve = machine.observe().first()
        assertThat(stateFromObserve).isEqualTo(machine.current)
    }

    @Test
    fun concurrent_transitions_serialized_by_mutex() = runTest {
        val results = mutableListOf<Result<RuntimeState>>()
        val jobs = (1..10).map { i ->
            launch {
                results.add(machine.transition(RuntimeState.Installed))
            }
        }
        jobs.forEach { it.join() }

        assertThat(results.count { it.isSuccess }).isAtLeast(1)
        assertThat(machine.current).isEqualTo(RuntimeState.Installed)
    }
}
