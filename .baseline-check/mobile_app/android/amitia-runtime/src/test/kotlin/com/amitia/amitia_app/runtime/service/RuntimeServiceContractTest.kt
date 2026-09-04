package com.amitia.amitia_app.runtime.service

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class RuntimeServiceContractTest {

    @Test
    fun actionStartHost_hasExpectedValue() {
        assertEquals(
            "com.amitia.amitia_app.runtime.action.START_HOST",
            RuntimeServiceContract.ACTION_START_HOST
        )
    }

    @Test
    fun actionStopHost_hasExpectedValue() {
        assertEquals(
            "com.amitia.amitia_app.runtime.action.STOP_HOST",
            RuntimeServiceContract.ACTION_STOP_HOST
        )
    }

    @Test
    fun notificationChannelId_hasExpectedValue() {
        assertEquals("runtime_service", RuntimeServiceContract.NOTIFICATION_CHANNEL_ID)
    }

    @Test
    fun notificationId_isStablePositiveValue() {
        assertTrue(RuntimeServiceContract.NOTIFICATION_ID > 0)
        assertEquals(0x52435541, RuntimeServiceContract.NOTIFICATION_ID)
    }

    @Test
    fun extraRequestId_hasExpectedValue() {
        assertEquals(
            "com.amitia.amitia_app.runtime.extra.REQUEST_ID",
            RuntimeServiceContract.EXTRA_REQUEST_ID
        )
    }
}
