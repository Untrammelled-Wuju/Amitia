package com.amitia.amitia_app.nativeprovider.share

object ShareConstants {
    const val MAX_RESOURCES = 10
    const val MAX_SINGLE_RESOURCE_BYTES = 100L * 1024L * 1024L
    const val MAX_TOTAL_BYTES = 250L * 1024L * 1024L
    const val MAX_SHARE_TEXT_BYTES = 1024 * 1024
    const val MAX_SUBJECT_BYTES = 8 * 1024
    const val MAX_CHOOSER_TITLE_BYTES = 256
    const val EXPORT_TTL_MINUTES = 15L
    const val SHARE_EXPORT_DIR = "share-export"
    const val ACTION_SEND = "android.intent.action.SEND"
    const val ACTION_SEND_MULTIPLE = "android.intent.action.SEND_MULTIPLE"
    const val EXTRA_TEXT = "android.intent.extra.TEXT"
    const val EXTRA_SUBJECT = "android.intent.extra.SUBJECT"
    const val EXTRA_STREAM = "android.intent.extra.STREAM"

    const val OP_STATUS = "share.status"
    const val OP_SEND = "share.send"
    const val OP_RECEIVE_PENDING = "share.receive.pending"
    const val OP_RECEIVE_CONSUME = "share.receive.consume"
}
