package com.amitia.runtime.linux

import kotlinx.serialization.Serializable

@Serializable
data class RootfsManifest(
    val version: String,
    val createdAt: String,
    val description: String? = null,
    val components: List<RootfsComponent>,
    val totalSize: Long,
    val checksumAlgorithm: String = "SHA-256"
) {

    fun findComponent(name: String): RootfsComponent? = components.firstOrNull { it.name == name }

    fun findComponentByFile(fileName: String): RootfsComponent? =
        components.firstOrNull { it.file == fileName }

    fun isCompatibleWith(target: String): Boolean = components.all { it.target == target }

    fun totalFilesSize(): Long = components.sumOf { it.size }
}

@Serializable
data class RootfsComponent(
    val name: String,
    val file: String,
    val size: Long,
    val sha256: String,
    val type: String,
    val target: String,
    val buildCommand: String? = null,
    val source: String? = null,
    val variant: String? = null
) {

    fun isExecutable(): Boolean = type in EXECUTABLE_TYPES

    companion object {
        private val EXECUTABLE_TYPES = setOf(
            "go-backend",
            "vector-db",
            "graph-db",
            "binary"
        )
    }
}
