package com.amitia.amitia_app.nativeprovider.share

import android.content.ClipData
import android.content.Context
import android.content.Intent
import android.net.Uri

internal class ShareIntentBuilder(private val context: Context) {

    fun buildTextIntent(text: String?, subject: String?, chooserTitle: String?): Intent {
        val sendIntent = Intent(Intent.ACTION_SEND).apply {
            type = "text/plain"
            if (!text.isNullOrNotBlank()) putExtra(Intent.EXTRA_TEXT, text)
            if (!subject.isNullOrNotBlank()) putExtra(Intent.EXTRA_SUBJECT, subject)
        }
        return createChooser(sendIntent, chooserTitle)
    }

    fun buildSingleResourceIntent(
        resource: ShareExportItem,
        text: String?,
        subject: String?,
        chooserTitle: String?,
    ): Intent {
        val sendIntent = Intent(Intent.ACTION_SEND).apply {
            type = normalizeMimeType(resource.mimeType)
            putExtra(Intent.EXTRA_STREAM, resource.contentUri)
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
            if (!text.isNullOrNotBlank()) putExtra(Intent.EXTRA_TEXT, text)
            if (!subject.isNullOrNotBlank()) putExtra(Intent.EXTRA_SUBJECT, subject)
            clipData = ClipData.newUri(
                context.contentResolver,
                "Amitia",
                resource.contentUri,
            )
        }
        return createChooser(sendIntent, chooserTitle)
    }

    fun buildMultipleResourceIntent(
        resources: List<ShareExportItem>,
        text: String?,
        subject: String?,
        chooserTitle: String?,
    ): Intent {
        if (resources.isEmpty()) {
            return buildTextIntent(text, subject, chooserTitle)
        }
        if (resources.size == 1) {
            return buildSingleResourceIntent(resources[0], text, subject, chooserTitle)
        }

        val uriList = ArrayList<Uri>(resources.size)
        for (r in resources) {
            uriList.add(r.contentUri)
        }

        val mimeType = mergeMimeTypes(resources.map { it.mimeType })

        val sendIntent = Intent(Intent.ACTION_SEND_MULTIPLE).apply {
            type = mimeType
            putParcelableArrayListExtra(Intent.EXTRA_STREAM, uriList)
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
            if (!text.isNullOrNotBlank()) putExtra(Intent.EXTRA_TEXT, text)
            if (!subject.isNullOrNotBlank()) putExtra(Intent.EXTRA_SUBJECT, subject)
        }
        return createChooser(sendIntent, chooserTitle)
    }

    private fun createChooser(sendIntent: Intent, title: String?): Intent {
        return if (!title.isNullOrNotBlank()) {
            Intent.createChooser(sendIntent, title)
        } else {
            Intent.createChooser(sendIntent, "Share via Amitia")
        }
    }

    private fun normalizeMimeType(mime: String?): String {
        if (mime.isNullOrBlank()) return "application/octet-stream"
        val trimmed = mime.trim().lowercase()
        if (trimmed.contains("\n") || trimmed.contains("\r") || trimmed.any { it.code < 32 }) {
            return "application/octet-stream"
        }
        return trimmed
    }

    private fun mergeMimeTypes(mimeTypes: List<String>): String {
        val normalized = mimeTypes.map { normalizeMimeType(it) }.toSet()
        if (normalized.isEmpty()) return "application/octet-stream"
        if (normalized.size == 1) return normalized.first()

        val prefixes = normalized.map { it.substringBefore("/") }.toSet()
        if (prefixes.size == 1) {
            val prefix = prefixes.first()
            return if (prefix == "image" || prefix == "video" || prefix == "audio") {
                "$prefix/*"
            } else {
                return "application/octet-stream"
            }
        }
        return "application/octet-stream"
    }

    private fun String?.isNullOrNotBlank(): Boolean = !this.isNullOrBlank()
}
