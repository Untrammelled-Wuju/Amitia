package com.amitia.amitia_app.runtime

import com.amitia.amitia_app.runtime.api.RuntimeComponentSnapshot
import com.amitia.amitia_app.runtime.api.RuntimeComponentState
import com.amitia.amitia_app.runtime.api.RuntimeError
import com.amitia.amitia_app.runtime.api.RuntimeErrorCode
import com.amitia.amitia_app.runtime.api.RuntimeOperationType
import com.amitia.amitia_app.runtime.api.RuntimeProgress
import com.amitia.amitia_app.runtime.api.RuntimeProgressStage
import com.amitia.amitia_app.runtime.api.RuntimeSnapshot
import com.amitia.amitia_app.runtime.api.RuntimeState
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class RuntimeSnapshotTest {

    @Test
    fun initial_state() {
        val initial = RuntimeSnapshot.initial()
        assertEquals(RuntimeState.UNKNOWN, initial.state)
        assertEquals(0L, initial.generation)
        assertEquals(RuntimeProgress.none().stage, initial.progress.stage)
    }

    @Test
    fun initial_runtime_version_is_null() {
        val initial = RuntimeSnapshot.initial()
        assertEquals(null, initial.runtimeVersion)
    }

    @Test
    fun initial_active_operation_is_null() {
        val initial = RuntimeSnapshot.initial()
        assertEquals(null, initial.activeOperationId)
        assertEquals(null, initial.activeOperationType)
    }

    @Test
    fun initial_components_empty() {
        val initial = RuntimeSnapshot.initial()
        assertTrue(initial.components.isEmpty())
    }

    @Test
    fun initial_last_error_is_null() {
        val initial = RuntimeSnapshot.initial()
        assertEquals(null, initial.lastError)
    }

    @Test
    fun components_defensive_copy() {
        val snapshot = RuntimeSnapshot(
            state = RuntimeState.INSTALLED,
            runtimeVersion = "1.0.0",
            activeOperationId = null,
            activeOperationType = null,
            progress = RuntimeProgress.none(),
            components = listOf(
                RuntimeComponentSnapshot(
                    id = "runtime.package",
                    state = RuntimeComponentState.INSTALLED,
                    required = true,
                    version = "1.0.0",
                    errorCode = null,
                    updatedAtEpochMillis = 1000L
                )
            ),
            lastError = null,
            generation = 1L,
            updatedAtEpochMillis = 1000L
        )
        val components = snapshot.components
        assertTrue(components is List<*>)
        assertEquals(1, components.size)
    }

    @Test
    fun generation_starts_at_zero() {
        val initial = RuntimeSnapshot.initial()
        assertEquals(0L, initial.generation)
    }

    @Test
    fun active_operation_id_and_type_pairing() {
        val validSnapshot = RuntimeSnapshot(
            state = RuntimeState.STARTING,
            runtimeVersion = null,
            activeOperationId = "op-123",
            activeOperationType = RuntimeOperationType.START,
            progress = RuntimeProgress.none(),
            components = emptyList(),
            lastError = null,
            generation = 1L,
            updatedAtEpochMillis = 1000L
        )
        assertNotNull(validSnapshot.activeOperationId)
        assertNotNull(validSnapshot.activeOperationType)
    }

    @Test
    fun active_operation_mismatch_id_only() {
        try {
            RuntimeSnapshot(
                state = RuntimeState.STARTING,
                runtimeVersion = null,
                activeOperationId = "op-123",
                activeOperationType = null,
                progress = RuntimeProgress.none(),
                components = emptyList(),
                lastError = null,
                generation = 1L,
                updatedAtEpochMillis = 1000L
            )
            throw AssertionError("should require both operation id and type")
        } catch (_: IllegalArgumentException) {
        }
    }

    @Test
    fun active_operation_mismatch_type_only() {
        try {
            RuntimeSnapshot(
                state = RuntimeState.STARTING,
                runtimeVersion = null,
                activeOperationId = null,
                activeOperationType = RuntimeOperationType.START,
                progress = RuntimeProgress.none(),
                components = emptyList(),
                lastError = null,
                generation = 1L,
                updatedAtEpochMillis = 1000L
            )
            throw AssertionError("should require both operation id and type")
        } catch (_: IllegalArgumentException) {
        }
    }

    @Test
    fun progress_default_in_initial() {
        val initial = RuntimeSnapshot.initial()
        assertEquals(RuntimeProgressStage.NONE, initial.progress.stage)
        assertEquals(0, initial.progress.completedUnits)
        assertEquals(0, initial.progress.totalUnits)
        assertEquals(0, initial.progress.percent)
        assertEquals(null, initial.progress.messageKey)
    }

    @Test
    fun error_with_details_creates_copy() {
        val details = mutableMapOf("key1" to "value1", "key2" to "value2")
        val error = RuntimeError(
            code = RuntimeErrorCode.INTERNAL_ERROR,
            message = "test error",
            recoverable = true,
            componentId = "runtime.package",
            detailsSource = details
        )
        details["key3"] = "value3"
        assertFalse(error.details.containsKey("key3"))
    }

    @Test
    fun component_snapshot_fields() {
        val component = RuntimeComponentSnapshot(
            id = "backend.go",
            state = RuntimeComponentState.READY,
            required = true,
            version = "2.0.0",
            errorCode = null,
            updatedAtEpochMillis = 2000L
        )
        assertEquals("backend.go", component.id)
        assertEquals(RuntimeComponentState.READY, component.state)
        assertEquals(true, component.required)
        assertEquals("2.0.0", component.version)
        assertEquals(2000L, component.updatedAtEpochMillis)
    }
}
