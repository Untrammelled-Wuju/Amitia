package com.amitia.core.database.converter

import androidx.room.TypeConverter
import kotlinx.serialization.builtins.ListSerializer
import kotlinx.serialization.builtins.MapSerializer
import kotlinx.serialization.builtins.serializer
import kotlinx.serialization.json.Json

class Converters {

    private val json = Json {
        ignoreUnknownKeys = true
        encodeDefaults = true
    }

    private val stringListSerializer = ListSerializer(String.serializer())
    private val stringMapSerializer = MapSerializer(String.serializer(), String.serializer())

    @TypeConverter
    fun stringListToJson(value: List<String>?): String? {
        if (value == null) return null
        return json.encodeToString(stringListSerializer, value)
    }

    @TypeConverter
    fun jsonToStringList(value: String?): List<String>? {
        if (value.isNullOrEmpty()) return emptyList()
        return runCatching { json.decodeFromString(stringListSerializer, value) }.getOrDefault(emptyList())
    }

    @TypeConverter
    fun stringMapToJson(value: Map<String, String>?): String? {
        if (value == null) return null
        return json.encodeToString(stringMapSerializer, value)
    }

    @TypeConverter
    fun jsonToStringMap(value: String?): Map<String, String>? {
        if (value.isNullOrEmpty()) return emptyMap()
        return runCatching { json.decodeFromString(stringMapSerializer, value) }.getOrDefault(emptyMap())
    }

    @TypeConverter
    fun intListToJson(value: List<Int>?): String? {
        if (value == null) return null
        return json.encodeToString(ListSerializer(Int.serializer()), value)
    }

    @TypeConverter
    fun jsonToIntList(value: String?): List<Int>? {
        if (value.isNullOrEmpty()) return emptyList()
        return runCatching { json.decodeFromString(ListSerializer(Int.serializer()), value) }.getOrDefault(emptyList())
    }
}
