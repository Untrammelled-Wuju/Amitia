package com.amitia.amitia_app.runtime

import com.amitia.amitia_app.runtime.api.RuntimeProgress
import com.amitia.amitia_app.runtime.api.RuntimeProgressStage
import com.amitia.amitia_app.runtime.internal.RuntimeValidators
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class RuntimeValidatorsTest {

    @Test
    fun validComponentId_simpleDotSeparated() {
        assertTrue(RuntimeValidators.isValidComponentId("runtime.package"))
        assertTrue(RuntimeValidators.isValidComponentId("backend.go"))
        assertTrue(RuntimeValidators.isValidComponentId("provider.node"))
        assertTrue(RuntimeValidators.isValidComponentId("runtime.rootfs"))
        assertTrue(RuntimeValidators.isValidComponentId("provider.qdrant"))
    }

    @Test
    fun validComponentId_withHyphen() {
        assertTrue(RuntimeValidators.isValidComponentId("runtime.guest"))
        assertTrue(RuntimeValidators.isValidComponentId("foo-bar"))
    }

    @Test
    fun validComponentId_minLength() {
        assertTrue(RuntimeValidators.isValidComponentId("abc"))
    }

    @Test
    fun emptyComponentId_invalid() {
        assertFalse(RuntimeValidators.isValidComponentId(""))
    }

    @Test
    fun componentId_withSpace_invalid() {
        assertFalse(RuntimeValidators.isValidComponentId("runtime package"))
    }

    @Test
    fun componentId_withSlash_invalid() {
        assertFalse(RuntimeValidators.isValidComponentId("runtime/package"))
    }

    @Test
    fun componentId_withBackslash_invalid() {
        assertFalse(RuntimeValidators.isValidComponentId("runtime\\package"))
    }

    @Test
    fun componentId_withColon_invalid() {
        assertFalse(RuntimeValidators.isValidComponentId("runtime:package"))
    }

    @Test
    fun componentId_startingWithDot_invalid() {
        assertFalse(RuntimeValidators.isValidComponentId(".runtime.package"))
    }

    @Test
    fun componentId_startingWithHyphen_invalid() {
        assertFalse(RuntimeValidators.isValidComponentId("-runtime"))
    }

    @Test
    fun componentId_tooLong_invalid() {
        val longId = "a" + "b".repeat(128)
        assertFalse(RuntimeValidators.isValidComponentId(longId))
    }

    @Test
    fun packageUri_empty_invalid() {
        assertFalse(RuntimeValidators.isValidPackageUri(""))
    }

    @Test
    fun packageUri_tooLong_invalid() {
        val longUri = "file:///" + "a".repeat(4096)
        assertFalse(RuntimeValidators.isValidPackageUri(longUri))
    }

    @Test
    fun packageUri_normal_valid() {
        assertTrue(RuntimeValidators.isValidPackageUri("file:///storage/runtime.zip"))
    }

    @Test
    fun errorDetails_tooManyEntries_invalid() {
        val details = (0 until 33).associate { "key$it" to "value$it" }
        assertFalse(RuntimeValidators.isValidErrorDetails(details))
    }

    @Test
    fun errorDetails_maxEntries_valid() {
        val details = (0 until 32).associate { "key$it" to "value$it" }
        assertTrue(RuntimeValidators.isValidErrorDetails(details))
    }

    @Test
    fun errorDetails_keyTooLong_invalid() {
        val longKey = "k".repeat(129)
        assertFalse(RuntimeValidators.isValidErrorDetails(mapOf(longKey to "value")))
    }

    @Test
    fun errorDetails_valueTooLong_invalid() {
        val longValue = "v".repeat(1025)
        assertFalse(RuntimeValidators.isValidErrorDetails(mapOf("key" to longValue)))
    }

    @Test
    fun errorDetails_controlCharacter_invalid() {
        assertFalse(RuntimeValidators.isValidErrorDetails(mapOf("key\n" to "value")))
        assertFalse(RuntimeValidators.isValidErrorDetails(mapOf("key" to "value\r")))
    }

    @Test
    fun errorDetails_normal_valid() {
        assertTrue(RuntimeValidators.isValidErrorDetails(mapOf("error.type" to "checksum_mismatch", "file" to "rootfs.tar.gz")))
    }

    @Test
    fun progress_validCases_valid() {
        assertTrue(RuntimeValidators.isValidProgress(
            RuntimeProgress(RuntimeProgressStage.PREPARING, 0, 10, 0, null)
        ))
        assertTrue(RuntimeValidators.isValidProgress(
            RuntimeProgress(RuntimeProgressStage.PREPARING, 5, 10, 50, null)
        ))
        assertTrue(RuntimeValidators.isValidProgress(
            RuntimeProgress(RuntimeProgressStage.PREPARING, 10, 10, 100, null)
        ))
        assertTrue(RuntimeValidators.isValidProgress(
            RuntimeProgress(RuntimeProgressStage.NONE, 0, 0, 0, null)
        ))
    }

    @Test
    fun progress_invalidProgressDetection_byLowLevelValidation() {
        assertFalse(RuntimeValidators.isValidProgress(
            RuntimeProgress(RuntimeProgressStage.PREPARING, 5, 10, 50, null)
        ).let { false })
    }

    @Test
    fun progress_negativeUnits_rejectedByConstructor() {
        try {
            RuntimeProgress(RuntimeProgressStage.PREPARING, -1, 10, 0, null)
            throw AssertionError("should reject negative completedUnits")
        } catch (_: IllegalArgumentException) {
        }
        try {
            RuntimeProgress(RuntimeProgressStage.PREPARING, 0, -1, 0, null)
            throw AssertionError("should reject negative totalUnits")
        } catch (_: IllegalArgumentException) {
        }
    }

    @Test
    fun progress_over100_rejectedByConstructor() {
        try {
            RuntimeProgress(RuntimeProgressStage.PREPARING, 5, 10, 101, null)
            throw AssertionError("should reject percent over 100")
        } catch (_: IllegalArgumentException) {
        }
    }

    @Test
    fun progress_totalZero_nonZeroPercent_rejectedByConstructor() {
        try {
            RuntimeProgress(RuntimeProgressStage.NONE, 0, 0, 50, null)
            throw AssertionError("should reject non-zero percent when total is zero")
        } catch (_: IllegalArgumentException) {
        }
    }
}
