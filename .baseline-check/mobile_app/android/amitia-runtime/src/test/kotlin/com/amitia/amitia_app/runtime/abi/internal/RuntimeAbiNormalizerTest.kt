package com.amitia.amitia_app.runtime.abi.internal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class RuntimeAbiNormalizerTest {

    @Test
    fun `normalize lowercase and trim`() {
        assertEquals("arm64-v8a", RuntimeAbiNormalizer.normalize("  ARM64-V8A  "))
    }

    @Test
    fun `normalize returns null for blank`() {
        assertNull(RuntimeAbiNormalizer.normalize("   "))
        assertNull(RuntimeAbiNormalizer.normalize(""))
    }

    @Test
    fun `normalize preserves arm64-v8a`() {
        assertEquals("arm64-v8a", RuntimeAbiNormalizer.normalize("arm64-v8a"))
    }

    @Test
    fun `normalizeList deduplicates preserving order`() {
        val result = RuntimeAbiNormalizer.normalizeList(listOf("arm64-v8a", "arm64-v8a", "x86", "x86"))
        assertEquals(listOf("arm64-v8a", "x86"), result)
    }

    @Test
    fun `normalizeList skips blank`() {
        val result = RuntimeAbiNormalizer.normalizeList(listOf("arm64-v8a", "   ", "x86"))
        assertEquals(listOf("arm64-v8a", "x86"), result)
    }

    @Test
    fun `normalizeList returns empty for empty input`() {
        assertEquals(emptyList<String>(), RuntimeAbiNormalizer.normalizeList(emptyList()))
    }

    @Test
    fun `normalize does not invent arm64-v8a from arm64`() {
        val result = RuntimeAbiNormalizer.normalize("arm64")
        assertEquals("arm64", result)
    }
}
