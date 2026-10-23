package com.amitia.amitia_app.update

import android.content.Context
import android.os.Environment
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File
import java.security.MessageDigest

@RunWith(AndroidJUnit4::class)
class SegmentedApkDownloadProductionTest {
    @Test
    fun downloadsProductionApkThroughForegroundService() {
        val context = ApplicationProvider.getApplicationContext<Context>()
        val url =
            "https://amitia.untrammelled.top/amitia/android/releases/19/" +
                "amitia-26.2.0-beta.13-release-arm64-v8a-20261023-082245.apk"
        val expectedSize = 136603431L
        val expectedSha256 =
            "8666a2c0d4d5309f38ff460a5f598ecead7fb2e4048fbaae0c092fbf135691ea"
        val downloadId = System.currentTimeMillis()
        val directory =
            context.getExternalFilesDir(Environment.DIRECTORY_DOWNLOADS)
                ?: File(context.filesDir, "downloads")
        directory.mkdirs()
        val target = File(directory, "instrumentation-production.apk")
        target.delete()
        AppUpdateDownloadStore.initialize(
            context,
            downloadId,
            url,
            target.name,
            expectedSize,
            expectedSha256,
            target.absolutePath,
        )
        SegmentedApkDownloadService.start(
            context,
            downloadId,
            url,
            target.name,
            expectedSize,
            expectedSha256,
        )
        val deadline = System.currentTimeMillis() + 30 * 60 * 1000
        var status = AppUpdateDownloadStore.status(context, downloadId)
        while ((status["statusCode"] as? Int) !in setOf(8, 16, -1) &&
            System.currentTimeMillis() < deadline
        ) {
            Thread.sleep(1000)
            status = AppUpdateDownloadStore.status(context, downloadId)
        }
        assertEquals(
            status["message"].toString(),
            8,
            (status["statusCode"] as? Int) ?: -1,
        )
        assertTrue(target.isFile)
        assertEquals(expectedSize, target.length())
        assertEquals(expectedSha256, sha256(target))
        target.delete()
    }

    private fun sha256(file: File): String {
        val digest = MessageDigest.getInstance("SHA-256")
        file.inputStream().use { input ->
            val buffer = ByteArray(128 * 1024)
            while (true) {
                val read = input.read(buffer)
                if (read <= 0) break
                digest.update(buffer, 0, read)
            }
        }
        return digest.digest().joinToString("") { "%02x".format(it) }
    }
}
