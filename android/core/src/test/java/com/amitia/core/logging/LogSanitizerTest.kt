package com.amitia.core.logging

import com.google.common.truth.Truth.assertThat
import org.junit.Test

class LogSanitizerTest {

    @Test
    fun redact_empty_string_unchanged() {
        assertThat(LogSanitizer.redact("")).isEqualTo("")
    }

    @Test
    fun redact_token_pattern_replaced_with_REDACTED() {
        val raw = "Authorization: Bearer abcdef1234567890abcdef1234567890"

        val redacted = LogSanitizer.redact(raw)

        assertThat(redacted).contains("[REDACTED]")
        assertThat(redacted).doesNotContain("abcdef1234567890abcdef1234567890")
    }

    @Test
    fun redact_token_with_equals_separator_replaced() {
        val raw = "access_token=abcdef1234567890abcdef1234567890"

        val redacted = LogSanitizer.redact(raw)

        assertThat(redacted).contains("[REDACTED]")
    }

    @Test
    fun redact_password_pattern_replaced() {
        val raw = "password=supersecret"

        val redacted = LogSanitizer.redact(raw)

        assertThat(redacted).contains("[REDACTED]")
        assertThat(redacted).doesNotContain("supersecret")
    }

    @Test
    fun redact_email_replaced_with_placeholder() {
        val raw = "user email is test@example.com end"

        val redacted = LogSanitizer.redact(raw)

        assertThat(redacted).contains("[EMAIL]")
        assertThat(redacted).doesNotContain("test@example.com")
    }

    @Test
    fun redact_phone_replaced_with_placeholder() {
        val raw = "phone: 13812345678 call"

        val redacted = LogSanitizer.redact(raw)

        assertThat(redacted).contains("[PHONE]")
        assertThat(redacted).doesNotContain("13812345678")
    }

    @Test
    fun redact_long_message_truncated_with_marker() {
        val raw = "x".repeat(500)

        val redacted = LogSanitizer.redact(raw)

        assertThat(redacted.length).isLessThan(raw.length)
        assertThat(redacted).contains("truncated")
    }

    @Test
    fun redact_short_message_kept_unchanged() {
        val raw = "starting backend on port 18899"

        val redacted = LogSanitizer.redact(raw)

        assertThat(redacted).isEqualTo("starting backend on port 18899")
    }

    @Test
    fun redactThrowable_includes_class_name_and_redacted_message() {
        val ex = IllegalStateException("Authorization: Bearer supersecret123456")

        val text = LogSanitizer.redactThrowable(ex)

        assertThat(text).contains("IllegalStateException")
        assertThat(text).contains("[REDACTED]")
        assertThat(text).doesNotContain("supersecret123456")
    }

    @Test
    fun redactThrowable_includes_top_5_stack_frames() {
        val ex = RuntimeException("boom")

        val text = LogSanitizer.redactThrowable(ex)

        val atCount = text.split(" at ").size - 1
        assertThat(atCount).isAtMost(5)
        assertThat(atCount).isGreaterThan(0)
    }

    @Test
    fun redact_api_key_pattern_replaced() {
        val raw = "api_key=sk-abcdef1234567890abcdef1234567890"

        val redacted = LogSanitizer.redact(raw)

        assertThat(redacted).contains("[REDACTED]")
    }
}
