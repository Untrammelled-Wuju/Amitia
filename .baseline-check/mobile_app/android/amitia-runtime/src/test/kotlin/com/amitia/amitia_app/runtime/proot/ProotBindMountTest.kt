package com.amitia.amitia_app.runtime.proot

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Test

class ProotBindMountTest {
    @Test fun valid_mount() { val m = ProotBindMount.create("/sdcard", "/mnt/sdcard"); assertEquals("/sdcard", m.host); assertEquals("/mnt/sdcard", m.guest) }
    @Test(expected = IllegalArgumentException::class) fun empty_host_fails() { ProotBindMount.create("", "/mnt") }
    @Test(expected = IllegalArgumentException::class) fun empty_guest_fails() { ProotBindMount.create("/host", "") }
    @Test(expected = IllegalArgumentException::class) fun relative_host_fails() { ProotBindMount.create("relative", "/mnt") }
    @Test(expected = IllegalArgumentException::class) fun relative_guest_fails() { ProotBindMount.create("/host", "relative") }
    @Test(expected = IllegalArgumentException::class) fun guest_root_fails() { ProotBindMount.create("/host", "/") }
    @Test(expected = IllegalArgumentException::class) fun traversal_fails() { ProotBindMount.create("/host", "/mnt/../etc") }
    @Test(expected = IllegalArgumentException::class) fun guest_colon_fails() { ProotBindMount.create("/host", "/mnt:extra") }
    @Test fun equality() { val a = ProotBindMount.create("/host", "/mnt"); val b = ProotBindMount.create("/host", "/mnt"); assertEquals(a, b) }
    @Test fun to_string() { val m = ProotBindMount.create("/host", "/mnt"); assertEquals("/host:/mnt", m.toString()) }
    @Test(expected = IllegalArgumentException::class) fun host_traversal_fails() { ProotBindMount.create("/host/../../etc", "/mnt") }
    @Test(expected = IllegalArgumentException::class) fun host_nul_fails() { ProotBindMount.create("/host\u0000", "/mnt") }
    @Test(expected = IllegalArgumentException::class) fun guest_nul_fails() { ProotBindMount.create("/host", "/mnt\u0000") }
    @Test(expected = IllegalArgumentException::class) fun host_escape_sdcard_fails() { ProotBindMount.create("/data/local/tmp/../../sdcard", "/mnt") }
    @Test fun valid_runtime_root_bind() { val m = ProotBindMount.create("/data/user/0/app/files/runtime", "/opt/amitia"); assertEquals("/data/user/0/app/files/runtime", m.host); assertEquals("/opt/amitia", m.guest) }

    @Test fun read_only_to_string_is_not_command_syntax() {
        val m = ProotBindMount.create("/host", "/mnt", readOnly = true)
        assertEquals("/host:/mnt [readOnly]", m.toString())
        assertFalse(m.toString().endsWith(":ro"))
    }
}