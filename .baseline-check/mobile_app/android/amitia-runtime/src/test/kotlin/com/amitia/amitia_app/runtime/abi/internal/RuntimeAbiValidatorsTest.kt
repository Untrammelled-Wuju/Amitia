package com.amitia.amitia_app.runtime.abi.internal

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class RuntimeAbiValidatorsTest {

    @Test
    fun `containsArm64 recognizes arm64-v8a`() {
        assertTrue(RuntimeAbiValidators.containsArm64(listOf("arm64-v8a")))
    }

    @Test
    fun `containsArm64 is case insensitive`() {
        assertTrue(RuntimeAbiValidators.containsArm64(listOf("ARM64-V8A")))
    }

    @Test
    fun `containsArm64 returns false for x86 list`() {
        assertFalse(RuntimeAbiValidators.containsArm64(listOf("x86", "x86_64")))
    }

    @Test
    fun `isForbiddenAbi rejects armeabi-v7a`() {
        assertTrue(RuntimeAbiValidators.isForbiddenAbi("armeabi-v7a"))
    }

    @Test
    fun `isForbiddenAbi rejects x86`() {
        assertTrue(RuntimeAbiValidators.isForbiddenAbi("x86"))
    }

    @Test
    fun `isForbiddenAbi rejects x86_64`() {
        assertTrue(RuntimeAbiValidators.isForbiddenAbi("x86_64"))
    }

    @Test
    fun `isForbiddenAbi allows arm64-v8a`() {
        assertFalse(RuntimeAbiValidators.isForbiddenAbi("arm64-v8a"))
    }

    @Test
    fun `isForbiddenAbi normalizes input`() {
        assertTrue(RuntimeAbiValidators.isForbiddenAbi("  X86  "))
    }

    @Test
    fun `containsArm64 fails for empty list`() {
        assertFalse(RuntimeAbiValidators.containsArm64(emptyList()))
    }

    @Test
    fun `isForbiddenAbi allows unknown`() {
        assertFalse(RuntimeAbiValidators.isForbiddenAbi("riscv64-unknown"))
    }
}
