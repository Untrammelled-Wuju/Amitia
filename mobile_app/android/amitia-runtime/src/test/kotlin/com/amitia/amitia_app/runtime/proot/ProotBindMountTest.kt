package com.amitia.amitia_app.runtime.proot

import org.junit.Assert.assertEquals
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
}