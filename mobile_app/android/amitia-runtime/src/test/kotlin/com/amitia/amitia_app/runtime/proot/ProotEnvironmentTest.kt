package com.amitia.amitia_app.runtime.proot

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class ProotEnvironmentTest {
    @Test fun empty_entries() { val e = ProotEnvironment.EMPTY; assertTrue(e.entries.isEmpty()) }
    @Test fun valid_entries() { val e = ProotEnvironment.of(mapOf("HOME" to "/root", "PATH" to "/bin")); assertEquals(2, e.entries.size); assertEquals("/root", e.toMap()["HOME"]) }
    @Test fun valid_lowercase_key() { val e = ProotEnvironment.of(mapOf("home" to "/root")); assertEquals("/root", e.toMap()["home"]) }
    @Test fun valid_mixed_case_key() { val e = ProotEnvironment.of(mapOf("HomeDir" to "/root")); assertEquals("/root", e.toMap()["HomeDir"]) }
    @Test(expected = IllegalArgumentException::class) fun digit_start_fails() { ProotEnvironment.of(mapOf("1INVALID" to "value")) }
    @Test(expected = IllegalArgumentException::class) fun empty_key_fails() { ProotEnvironment.of(mapOf("" to "value")) }
    @Test fun empty_value_ok() { val e = ProotEnvironment.of(mapOf("KEY" to "")); assertEquals("", e.toMap()["KEY"]) }
    @Test fun to_map() { val e = ProotEnvironment.of(listOf("HOME" to "/root", "PATH" to "/bin")); val m = e.toMap(); assertEquals("/root", m["HOME"]); assertEquals("/bin", m["PATH"]) }
}
