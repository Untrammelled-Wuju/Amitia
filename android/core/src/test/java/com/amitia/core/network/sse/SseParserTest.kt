package com.amitia.core.network.sse

import com.google.common.truth.Truth.assertThat
import org.junit.Test

class SseParserTest {

    @Test
    fun parses_single_event_with_event_and_data() {
        val raw = "event: token\ndata: {\"text\":\"hello\"}\n\n"

        val events = SseParser.parseEvents(raw)

        assertThat(events).hasSize(1)
        val event = events.first()
        assertThat(event.event).isEqualTo("token")
        assertThat(event.data).isEqualTo("{\"text\":\"hello\"}")
        assertThat(event.id).isNull()
    }

    @Test
    fun parses_multiple_data_lines_concatenated_with_newline() {
        val raw = "event: token\ndata: line1\ndata: line2\ndata: line3\n\n"

        val events = SseParser.parseEvents(raw)

        assertThat(events).hasSize(1)
        assertThat(events.first().data).isEqualTo("line1\nline2\nline3")
    }

    @Test
    fun parses_id_field() {
        val raw = "id: 42\ndata: payload\n\n"

        val events = SseParser.parseEvents(raw)

        assertThat(events).hasSize(1)
        assertThat(events.first().id).isEqualTo("42")
        assertThat(events.first().event).isEqualTo(SseEvent.EVENT_DEFAULT)
    }

    @Test
    fun parses_comment_lines_ignored() {
        val raw = ": this is a comment\nevent: ping\ndata: {}\n\n"

        val events = SseParser.parseEvents(raw)

        assertThat(events).hasSize(1)
        assertThat(events.first().event).isEqualTo("ping")
        assertThat(events.first().data).isEqualTo("{}")
    }

    @Test
    fun uses_default_event_name_when_missing() {
        val raw = "data: only-data\n\n"

        val events = SseParser.parseEvents(raw)

        assertThat(events).hasSize(1)
        assertThat(events.first().event).isEqualTo(SseEvent.EVENT_DEFAULT)
        assertThat(events.first().data).isEqualTo("only-data")
    }

    @Test
    fun parses_multiple_events_in_one_chunk() {
        val raw = "event: message_start\ndata: {}\n\nevent: token\ndata: {\"token\":\"a\"}\n\nevent: message_end\ndata: {}\n\n"

        val events = SseParser.parseEvents(raw)

        assertThat(events).hasSize(3)
        assertThat(events[0].event).isEqualTo("message_start")
        assertThat(events[1].event).isEqualTo("token")
        assertThat(events[2].event).isEqualTo("message_end")
    }

    @Test
    fun returns_empty_list_for_blank_input() {
        val events = SseParser.parseEvents("   ")

        assertThat(events).isEmpty()
    }

    @Test
    fun returns_null_for_block_with_only_comments() {
        val event = SseParser.parseBlock(": only comment\n: another")

        assertThat(event).isNull()
    }

    @Test
    fun handles_colon_without_space_after_field() {
        val raw = "data:no-space\nevent:no-space-event\n\n"

        val events = SseParser.parseEvents(raw)

        assertThat(events).hasSize(1)
        assertThat(events.first().event).isEqualTo("no-space-event")
        assertThat(events.first().data).isEqualTo("no-space")
    }

    @Test
    fun terminal_event_detected_correctly() {
        val raw = "event: message_end\ndata: {}\n\n"

        val events = SseParser.parseEvents(raw)

        assertThat(events.first().isTerminal()).isTrue()
    }
}
