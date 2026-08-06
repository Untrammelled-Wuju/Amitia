package com.amitia.amitia_app.runtime.proot

class ProotEnvironment private constructor(val entriesSource: List<Pair<String, String>>) {
    val entries: List<Pair<String, String>> = entriesSource.toList()
    fun toMap(): Map<String, String> = entries.toMap()
    companion object {
        val EMPTY: ProotEnvironment = ProotEnvironment(emptyList())
        fun of(entries: Map<String, String>): ProotEnvironment {
            for ((key, value) in entries) require(key.matches(Regex("^[A-Z_][A-Z0-9_]*$"))) { "invalid key: $key" }
            return ProotEnvironment(entries.toList())
        }
        fun of(entries: List<Pair<String, String>>): ProotEnvironment {
            for ((key, value) in entries) require(key.matches(Regex("^[A-Z_][A-Z0-9_]*$"))) { "invalid key: $key" }
            return ProotEnvironment(entries.toList())
        }
    }
}