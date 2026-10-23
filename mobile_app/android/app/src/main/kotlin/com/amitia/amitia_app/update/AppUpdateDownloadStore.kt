package com.amitia.amitia_app.update

import android.content.Context
import java.io.File

object AppUpdateDownloadStore {
    private const val PREFS_NAME = "app_update_downloads"
    private const val STATUS_PENDING = 1
    private const val STATUS_RUNNING = 2
    private const val STATUS_SUCCESSFUL = 8
    private const val STATUS_FAILED = 16

    fun initialize(
        context: Context,
        downloadId: Long,
        url: String,
        fileName: String,
        expectedSize: Long,
        expectedSha256: String,
        outputPath: String,
    ) {
        prefs(context).edit()
            .putString(key(downloadId, "url"), url)
            .putString(key(downloadId, "fileName"), fileName)
            .putLong(key(downloadId, "expectedSize"), expectedSize)
            .putString(key(downloadId, "expectedSha256"), expectedSha256)
            .putString(key(downloadId, "outputPath"), outputPath)
            .putInt(key(downloadId, "statusCode"), STATUS_PENDING)
            .putString(key(downloadId, "status"), "pending")
            .putLong(key(downloadId, "downloadedBytes"), 0L)
            .putLong(key(downloadId, "totalBytes"), expectedSize)
            .putInt(key(downloadId, "reasonCode"), 0)
            .putString(key(downloadId, "message"), "")
            .apply()
    }

    fun markRunning(context: Context, downloadId: Long) {
        prefs(context).edit()
            .putInt(key(downloadId, "statusCode"), STATUS_RUNNING)
            .putString(key(downloadId, "status"), "running")
            .apply()
    }

    fun updateProgress(context: Context, downloadId: Long, downloaded: Long, total: Long) {
        prefs(context).edit()
            .putInt(key(downloadId, "statusCode"), STATUS_RUNNING)
            .putString(key(downloadId, "status"), "running")
            .putLong(key(downloadId, "downloadedBytes"), downloaded)
            .putLong(key(downloadId, "totalBytes"), total)
            .apply()
    }

    fun markSucceeded(context: Context, downloadId: Long, file: File, total: Long) {
        prefs(context).edit()
            .putInt(key(downloadId, "statusCode"), STATUS_SUCCESSFUL)
            .putString(key(downloadId, "status"), "successful")
            .putLong(key(downloadId, "downloadedBytes"), total)
            .putLong(key(downloadId, "totalBytes"), total)
            .putInt(key(downloadId, "reasonCode"), 0)
            .putString(key(downloadId, "localUri"), file.toURI().toString())
            .putString(key(downloadId, "message"), "")
            .apply()
    }

    fun markFailed(context: Context, downloadId: Long, message: String, reasonCode: Int = 1000) {
        prefs(context).edit()
            .putInt(key(downloadId, "statusCode"), STATUS_FAILED)
            .putString(key(downloadId, "status"), "failed")
            .putInt(key(downloadId, "reasonCode"), reasonCode)
            .putString(key(downloadId, "message"), message)
            .apply()
    }

    fun status(context: Context, downloadId: Long): Map<String, Any?> {
        val values = prefs(context)
        return mapOf(
            "statusCode" to values.getInt(key(downloadId, "statusCode"), -1),
            "status" to values.getString(key(downloadId, "status"), "missing").orEmpty(),
            "downloadedBytes" to values.getLong(key(downloadId, "downloadedBytes"), 0L),
            "totalBytes" to values.getLong(key(downloadId, "totalBytes"), 0L),
            "reasonCode" to values.getInt(key(downloadId, "reasonCode"), 0),
            "localUri" to values.getString(key(downloadId, "localUri"), "").orEmpty(),
            "message" to values.getString(key(downloadId, "message"), "").orEmpty(),
        )
    }

    fun localFile(context: Context, downloadId: Long): File? {
        val path = prefs(context).getString(key(downloadId, "outputPath"), "").orEmpty()
        return path.takeIf { it.isNotEmpty() }?.let(::File)
    }

    fun outputPath(context: Context, downloadId: Long): String =
        prefs(context).getString(key(downloadId, "outputPath"), "").orEmpty()

    fun expectedSize(context: Context, downloadId: Long): Long =
        prefs(context).getLong(key(downloadId, "expectedSize"), 0L)

    fun expectedSha256(context: Context, downloadId: Long): String =
        prefs(context).getString(key(downloadId, "expectedSha256"), "").orEmpty()

    private fun prefs(context: Context) =
        context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)

    private fun key(downloadId: Long, name: String) = "$downloadId.$name"
}
