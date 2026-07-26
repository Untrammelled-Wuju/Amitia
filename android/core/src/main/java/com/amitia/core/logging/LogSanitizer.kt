package com.amitia.core.logging

object LogSanitizer {

    private val tokenPattern = Regex("(?i)(token|access[-_]?token|refresh[-_]?token|api[-_]?key|secret|authorization|bearer)\\s*[:=]\\s*(bearer\\s+)?[A-Za-z0-9._\\-+/=]{4,}")
    private val passwordPattern = Regex("(?i)(password|passwd|pwd)\\s*[:=]\\s*\\S+")
    private val emailPattern = Regex("[A-Za-z0-9._%+\\-]+@[A-Za-z0-9.\\-]+\\.[A-Za-z]{2,}")
    private val phonePattern = Regex("(?<!\\d)(1[3-9]\\d{9})(?!\\d)")
    private val longContentPattern = Regex("[\\s\\S]{240,}")

    fun redact(message: String): String {
        if (message.isEmpty()) return message
        var result = message
        result = tokenPattern.replace(result) { match ->
            val prefix = match.value.substringBeforeLast(':', match.value.substringBeforeLast('=', ""))
            "$prefix=[REDACTED]"
        }
        result = passwordPattern.replace(result) { match ->
            val key = match.value.substringBefore(':').substringBefore('=').trim()
            "$key=[REDACTED]"
        }
        result = emailPattern.replace(result, "[EMAIL]")
        result = phonePattern.replace(result, "[PHONE]")
        if (result.length > 200) {
            result = result.take(180) + "...[truncated ${result.length - 180} chars]"
        }
        return result
    }

    fun redactThrowable(throwable: Throwable): String {
        val builder = StringBuilder()
        builder.append(throwable::class.simpleName ?: "Throwable")
        builder.append(": ")
        builder.append(redact(throwable.message ?: ""))
        builder.append("\n")
        throwable.stackTrace.take(5).forEach { element ->
            builder.append("  at $element\n")
        }
        return builder.toString()
    }
}
