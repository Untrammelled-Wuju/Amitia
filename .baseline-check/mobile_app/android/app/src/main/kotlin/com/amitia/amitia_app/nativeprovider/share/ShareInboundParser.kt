package com.amitia.amitia_app.nativeprovider.share

import android.content.Intent
import android.net.Uri
import java.util.UUID

internal class ShareInboundParser {

    private val supportedMimeTypes = setOf(
        "text/plain",
        "image/*",
        "image/jpeg",
        "image/png",
        "image/gif",
        "image/webp",
        "application/pdf",
    )

    fun parse(intent: Intent?): IncomingShare? {
        if (intent == null) return null
        val action = intent.action ?: return null
        val type = intent.type ?: return null

        if (!isSupportedMimeType(type)) return null

        return when (action) {
            Intent.ACTION_SEND -> parseSingle(intent, type)
            Intent.ACTION_SEND_MULTIPLE -> parseMultiple(intent, type)
            else -> null
        }
    }

    private fun parseSingle(intent: Intent, type: String): IncomingShare? {
        val text = intent.getStringExtra(Intent.EXTRA_TEXT)
        val subject = intent.getStringExtra(Intent.EXTRA_SUBJECT)
        val uri: Uri? = intent.getParcelableExtra(Intent.EXTRA_STREAM)

        val resources = mutableListOf<SharedIncomingResource>()
        if (uri != null) {
            resources.add(
                SharedIncomingResource(
                    resourceUri = uri.toString(),
                    mimeType = type,
                )
            )
        }

        return IncomingShare(
            shareId = "inbound_${UUID.randomUUID().toString().take(12)}",
            text = text,
            subject = subject,
            resources = resources,
            receivedAt = System.currentTimeMillis(),
        )
    }

    private fun parseMultiple(intent: Intent, type: String): IncomingShare? {
        val text = intent.getStringExtra(Intent.EXTRA_TEXT)
        val subject = intent.getStringExtra(Intent.EXTRA_SUBJECT)
        val uris: ArrayList<Uri>? = intent.getParcelableArrayListExtra(Intent.EXTRA_STREAM)

        if (uris.isNullOrEmpty()) return null

        val resources = uris.map { uri ->
            SharedIncomingResource(
                resourceUri = uri.toString(),
                mimeType = type,
            )
        }

            if (resources.size > ShareConstants.MAX_RESOURCES) return null

            return IncomingShare(
            shareId = "inbound_${UUID.randomUUID().toString().take(12)}",
            text = text,
            subject = subject,
            resources = resources,
            receivedAt = System.currentTimeMillis(),
        )
    }

    private fun isSupportedMimeType(mime: String): Boolean {
        val normalized = mime.trim().lowercase()
        return supportedMimeTypes.contains(normalized) ||
            supportedMimeTypes.contains(normalized.substringBefore("/") + "/*")
    }
}
