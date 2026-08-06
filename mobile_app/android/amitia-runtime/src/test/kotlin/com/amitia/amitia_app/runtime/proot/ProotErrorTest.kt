package com.amitia.amitia_app.runtime.proot

import org.junit.Assert.assertEquals
import org.junit.Test

class ProotErrorTest {
    @Test fun of_creates_error() { val e = ProotError.of(ProotErrorCode.BINARY_NOT_FOUND, "not found"); assertEquals(ProotErrorCode.BINARY_NOT_FOUND, e.code); assertEquals("not found", e.message) }
    @Test fun details_are_isolated() {
        val params = mutableMapOf("key1" to "val1", "key2" to "val2")
        val e = ProotError(ProotErrorCode.INTERNAL_ERROR, "test", params)
        params["key3"] = "val3"
        org.junit.Assert.assertFalse(e.details.containsKey("key3"))
    }
}