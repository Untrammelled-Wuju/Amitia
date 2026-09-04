package com.amitia.amitia_app.runtime.abi.internal

internal object RuntimeAbiNormalizer {
    fun normalize(input: String): String? {
        val trimmed = input.trim().lowercase(java.util.Locale.ROOT)
        return if (trimmed.isEmpty()) null else trimmed
    }

    fun normalizeList(inputs: List<String>): List<String> {
        val seen = LinkedHashSet<String>()
        for (item in inputs) {
            val normalized = normalize(item) ?: continue
            seen.add(normalized)
        }
        return seen.toList()
    }
}