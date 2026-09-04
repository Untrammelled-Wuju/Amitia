package com.amitia.amitia_app.runtime.proot.internal

internal object SensitiveValueRedactor {
    private val PATTERNS = listOf(Regex("TOKEN", RegexOption.IGNORE_CASE), Regex("SECRET", RegexOption.IGNORE_CASE), Regex("PASSWORD", RegexOption.IGNORE_CASE), Regex("API_KEY", RegexOption.IGNORE_CASE), Regex("AUTH", RegexOption.IGNORE_CASE))
    fun isSensitiveKey(key: String): Boolean = PATTERNS.any { it.containsMatchIn(key) }
    fun redactMap(map: Map<String, String>): Map<String, String> {
        val result = LinkedHashMap<String, String>(map.size)
        for ((key, value) in map) result[key] = if (isSensitiveKey(key)) "***REDACTED***" else value
        return result
    }
    fun redactArguments(args: List<String>): List<String> = args.map {
        val idx = it.indexOf('=')
        if (idx > 0 && isSensitiveKey(it.substring(0, idx))) "${it.substring(0, idx)}=***REDACTED***" else it
    }
}