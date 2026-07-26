package com.amitia.core.network.sse

object SseParser {

    fun parseEvents(rawText: String): List<SseEvent> {
        val events = mutableListOf<SseEvent>()
        if (rawText.isBlank()) return events

        val blocks = rawText.split(BLOCK_DELIMITER)
        for (block in blocks) {
            val event = parseBlock(block) ?: continue
            events.add(event)
        }
        return events
    }

    fun parseBlock(block: String): SseEvent? {
        if (block.isBlank()) return null

        var event: String? = null
        val dataBuilder = StringBuilder()
        var id: String? = null

        val lines = block.split(LINE_DELIMITER)
        for (rawLine in lines) {
            if (rawLine.isBlank()) continue
            if (rawLine.startsWith(COMMENT_PREFIX)) continue

            val colonIndex = rawLine.indexOf(COLON)
            val field = if (colonIndex == -1) {
                rawLine
            } else {
                rawLine.substring(0, colonIndex)
            }
            val value = if (colonIndex == -1) {
                ""
            } else if (colonIndex + 1 < rawLine.length && rawLine[colonIndex + 1] == SPACE) {
                rawLine.substring(colonIndex + 2)
            } else {
                rawLine.substring(colonIndex + 1)
            }

            when (field) {
                SseEvent.FIELD_EVENT -> event = value
                SseEvent.FIELD_DATA -> {
                    if (dataBuilder.isNotEmpty()) {
                        dataBuilder.append(LINE_FEED)
                    }
                    dataBuilder.append(value)
                }
                SseEvent.FIELD_ID -> id = value
            }
        }

        val eventName = event ?: SseEvent.EVENT_DEFAULT
        val data = dataBuilder.toString()
        if (eventName == SseEvent.EVENT_DEFAULT && data.isEmpty() && id == null) {
            return null
        }

        return SseEvent(event = eventName, data = data, id = id)
    }

    private const val BLOCK_DELIMITER = "\n\n"
    private const val LINE_DELIMITER = "\n"
    private const val LINE_FEED = "\n"
    private const val COLON = ':'
    private const val SPACE = ' '
    private const val COMMENT_PREFIX = ':'
}
