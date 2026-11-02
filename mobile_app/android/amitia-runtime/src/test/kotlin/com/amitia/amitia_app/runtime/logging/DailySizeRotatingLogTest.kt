package com.amitia.amitia_app.runtime.logging

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File
import java.util.Calendar

class DailySizeRotatingLogTest {
    @Test
    fun write_rotatesBySize() {
        val directory = createDirectory()
        val timestamp = timestamp(2026, Calendar.NOVEMBER, 1, 12, 0)
        val writer = DailySizeRotatingLog(
            directory = directory,
            prefix = "mobile",
            maxFileBytes = 10,
            now = { timestamp },
        )
        writer.write("12345\n")
        writer.write("67890\n")
        writer.close()

        val date = DailySizeRotatingLog.dateKey(timestamp)
        assertEquals("12345\n", File(directory, "mobile.$date.001.log").readText())
        assertEquals("67890\n", File(directory, "mobile.$date.002.log").readText())
    }

    @Test
    fun write_rotatesByDate() {
        val directory = createDirectory()
        var timestamp = timestamp(2026, Calendar.NOVEMBER, 1, 23, 59)
        val writer = DailySizeRotatingLog(
            directory = directory,
            prefix = "mobile",
            now = { timestamp },
        )
        writer.write("day-one\n")
        timestamp = timestamp(2026, Calendar.NOVEMBER, 2, 0, 1)
        writer.write("day-two\n")
        writer.close()

        assertTrue(File(directory, "mobile.20261101.001.log").exists())
        assertTrue(File(directory, "mobile.20261102.001.log").exists())
    }

    @Test
    fun constructor_removesExpiredManagedLogsOnly() {
        val directory = createDirectory()
        val timestamp = timestamp(2026, Calendar.NOVEMBER, 1, 12, 0)
        val expired = File(directory, "mobile.20260901.001.log").apply {
            writeText("expired")
            setLastModified(timestamp - DailySizeRotatingLog.RETENTION_MS - 1000)
        }
        val recent = File(directory, "mobile.20261031.001.log").apply {
            writeText("recent")
            setLastModified(timestamp - 1000)
        }
        val legacy = File(directory, "platform.log").apply {
            writeText("expired")
            setLastModified(timestamp - DailySizeRotatingLog.RETENTION_MS - 1000)
        }
        val unrelated = File(directory, "mobile.json").apply { writeText("unrelated") }

        DailySizeRotatingLog(
            directory = directory,
            prefix = "mobile",
            now = { timestamp },
        ).close()

        assertFalse(expired.exists())
        assertFalse(legacy.exists())
        assertTrue(recent.exists())
        assertTrue(unrelated.exists())
    }

    private fun createDirectory(): File {
        val directory = kotlin.io.path.createTempDirectory("amitia-log-").toFile()
        directory.deleteOnExit()
        return directory
    }

    private fun timestamp(
        year: Int,
        month: Int,
        day: Int,
        hour: Int,
        minute: Int,
    ): Long {
        return Calendar.getInstance().apply {
            clear()
            set(year, month, day, hour, minute)
        }.timeInMillis
    }
}
