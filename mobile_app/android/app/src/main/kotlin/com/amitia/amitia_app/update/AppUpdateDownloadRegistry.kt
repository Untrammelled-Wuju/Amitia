package com.amitia.amitia_app.update

import android.content.Context

object AppUpdateDownloadRegistry {
    private const val PREFS_NAME = "app_update_system_download"
    private const val DOWNLOAD_ID = "downloadId"
    private const val URL = "url"
    private const val FILE_NAME = "fileName"
    private const val EXPECTED_SIZE = "expectedSize"
    private const val EXPECTED_SHA256 = "expectedSha256"

    data class ActiveDownload(
        val downloadId: Long,
        val url: String,
        val fileName: String,
        val expectedSize: Long,
        val expectedSha256: String,
    ) {
        fun matches(url: String, expectedSize: Long, expectedSha256: String): Boolean =
            this.url == url &&
                this.expectedSize == expectedSize &&
                this.expectedSha256 == expectedSha256
    }

    fun active(context: Context): ActiveDownload? {
        val values = prefs(context)
        val downloadId = values.getLong(DOWNLOAD_ID, -1L)
        val url = values.getString(URL, "").orEmpty()
        val fileName = values.getString(FILE_NAME, "").orEmpty()
        val expectedSize = values.getLong(EXPECTED_SIZE, -1L)
        val expectedSha256 = values.getString(EXPECTED_SHA256, "").orEmpty()
        if (downloadId <= 0L || url.isEmpty() || fileName.isEmpty() || expectedSize <= 0L ||
            !expectedSha256.matches(Regex("[a-f0-9]{64}"))
        ) {
            return null
        }
        return ActiveDownload(
            downloadId,
            url,
            fileName,
            expectedSize,
            expectedSha256,
        )
    }

    fun save(
        context: Context,
        downloadId: Long,
        url: String,
        fileName: String,
        expectedSize: Long,
        expectedSha256: String,
    ) {
        prefs(context).edit()
            .putLong(DOWNLOAD_ID, downloadId)
            .putString(URL, url)
            .putString(FILE_NAME, fileName)
            .putLong(EXPECTED_SIZE, expectedSize)
            .putString(EXPECTED_SHA256, expectedSha256)
            .apply()
    }

    fun clear(context: Context) {
        prefs(context).edit().clear().apply()
    }

    private fun prefs(context: Context) =
        context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
}
