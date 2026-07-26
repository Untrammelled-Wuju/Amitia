package com.amitia.core.network.sse

data class SseEvent(
    val event: String,
    val data: String,
    val id: String? = null
) {

    fun isTerminal(): Boolean = event == EVENT_TERMINAL || event == EVENT_ERROR

    fun isHeartbeat(): Boolean = event == EVENT_PING

    companion object {
        const val EVENT_DEFAULT = "message"
        const val EVENT_TERMINAL = "message_end"
        const val EVENT_ERROR = "error"
        const val EVENT_PING = "ping"

        const val EVENT_MESSAGE_START = "message_start"
        const val EVENT_TOKEN = "token"
        const val EVENT_VOICE_AUDIO = "voice_audio"

        const val EVENT_MESSAGE_CREATED = "message_created"
        const val EVENT_MESSAGE_UPDATED = "message_updated"
        const val EVENT_CONVERSATION_UPDATED = "conversation_updated"
        const val EVENT_PROACTIVE_MESSAGE = "proactive_message"

        const val FIELD_EVENT = "event"
        const val FIELD_DATA = "data"
        const val FIELD_ID = "id"
    }
}
