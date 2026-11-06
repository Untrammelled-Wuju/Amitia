package com.amitia.amitia_app.update

import android.app.DownloadManager
import android.content.Context
import android.net.Uri
import android.os.Environment

class AppUpdateSystemDownloader(context: Context) {
    private val appContext = context.applicationContext
    private val manager =
        appContext.getSystemService(DownloadManager::class.java)

    fun enqueue(
        url: String,
        fileName: String,
        title: String,
    ): Long {
        val request = DownloadManager.Request(Uri.parse(url))
            .setTitle(title)
            .setDescription(fileName)
            .setMimeType(APK_MIME_TYPE)
            .setNotificationVisibility(
                DownloadManager.Request.VISIBILITY_VISIBLE_NOTIFY_COMPLETED,
            )
            .setAllowedOverMetered(true)
            .setAllowedOverRoaming(false)
            .setDestinationInExternalFilesDir(
                appContext,
                Environment.DIRECTORY_DOWNLOADS,
                fileName,
            )
            .addRequestHeader("Accept-Encoding", "identity")
            .addRequestHeader("Cache-Control", "no-cache")
        return manager.enqueue(request)
    }

    fun status(downloadId: Long): Map<String, Any?> {
        manager.query(DownloadManager.Query().setFilterById(downloadId)).use { cursor ->
            if (!cursor.moveToFirst()) {
                return missingStatus(downloadId)
            }
            val statusCode = cursor.getInt(cursor.getColumnIndexOrThrow(DownloadManager.COLUMN_STATUS))
            val reasonCode = cursor.getInt(cursor.getColumnIndexOrThrow(DownloadManager.COLUMN_REASON))
            val downloadedBytes = cursor
                .getLong(cursor.getColumnIndexOrThrow(DownloadManager.COLUMN_BYTES_DOWNLOADED_SO_FAR))
                .coerceAtLeast(0L)
            val totalBytes = cursor
                .getLong(cursor.getColumnIndexOrThrow(DownloadManager.COLUMN_TOTAL_SIZE_BYTES))
                .coerceAtLeast(0L)
            val localUri = downloadedUri(downloadId).orEmpty()
            val message = statusMessage(statusCode, reasonCode)
            return mapOf(
                "downloadId" to downloadId,
                "statusCode" to statusCode,
                "status" to statusName(statusCode),
                "downloadedBytes" to downloadedBytes,
                "totalBytes" to totalBytes,
                "reasonCode" to reasonCode,
                "reason" to reasonName(reasonCode),
                "localUri" to localUri,
                "message" to message,
            )
        }
    }

    fun downloadedUri(downloadId: Long): String {
        return manager.getUriForDownloadedFile(downloadId)?.toString().orEmpty()
    }

    fun remove(downloadId: Long) {
        manager.remove(downloadId)
    }

    private fun missingStatus(downloadId: Long): Map<String, Any?> =
        mapOf(
            "downloadId" to downloadId,
            "statusCode" to -1,
            "status" to "missing",
            "downloadedBytes" to 0L,
            "totalBytes" to 0L,
            "reasonCode" to 0,
            "reason" to "DownloadManager record not found",
            "localUri" to "",
            "message" to "DownloadManager record not found for downloadId=$downloadId",
        )

    private fun statusMessage(statusCode: Int, reasonCode: Int): String {
        if (statusCode != DownloadManager.STATUS_FAILED &&
            statusCode != DownloadManager.STATUS_PAUSED
        ) {
            return ""
        }
        return "DownloadManager status=$statusCode reasonCode=$reasonCode reason=${reasonName(reasonCode)}"
    }

    private fun statusName(statusCode: Int): String = when (statusCode) {
        DownloadManager.STATUS_PENDING -> "pending"
        DownloadManager.STATUS_RUNNING -> "running"
        DownloadManager.STATUS_PAUSED -> "paused"
        DownloadManager.STATUS_SUCCESSFUL -> "successful"
        DownloadManager.STATUS_FAILED -> "failed"
        else -> "unknown"
    }

    private fun reasonName(reasonCode: Int): String = when (reasonCode) {
        DownloadManager.ERROR_UNKNOWN -> "ERROR_UNKNOWN"
        DownloadManager.ERROR_FILE_ALREADY_EXISTS -> "ERROR_FILE_ALREADY_EXISTS"
        DownloadManager.ERROR_FILE_ERROR -> "ERROR_FILE_ERROR"
        DownloadManager.ERROR_HTTP_DATA_ERROR -> "ERROR_HTTP_DATA_ERROR"
        DownloadManager.ERROR_UNHANDLED_HTTP_CODE -> "ERROR_UNHANDLED_HTTP_CODE"
        DownloadManager.ERROR_TOO_MANY_REDIRECTS -> "ERROR_TOO_MANY_REDIRECTS"
        DownloadManager.ERROR_INSUFFICIENT_SPACE -> "ERROR_INSUFFICIENT_SPACE"
        DownloadManager.ERROR_DEVICE_NOT_FOUND -> "ERROR_DEVICE_NOT_FOUND"
        DownloadManager.ERROR_CANNOT_RESUME -> "ERROR_CANNOT_RESUME"
        DownloadManager.PAUSED_WAITING_TO_RETRY -> "PAUSED_WAITING_TO_RETRY"
        DownloadManager.PAUSED_WAITING_FOR_NETWORK -> "PAUSED_WAITING_FOR_NETWORK"
        DownloadManager.PAUSED_QUEUED_FOR_WIFI -> "PAUSED_QUEUED_FOR_WIFI"
        DownloadManager.PAUSED_UNKNOWN -> "PAUSED_UNKNOWN"
        else -> "UNKNOWN_REASON"
    }

    companion object {
        const val APK_MIME_TYPE = "application/vnd.android.package-archive"
    }
}
