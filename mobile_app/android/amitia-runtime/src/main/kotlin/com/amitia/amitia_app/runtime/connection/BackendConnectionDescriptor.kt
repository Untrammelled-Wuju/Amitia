package com.amitia.amitia_app.runtime.connection

data class BackendConnectionDescriptor(
    val schemaVersion: Int,
    val generation: Long,
    val host: String,
    val port: Int,
    val httpScheme: String,
    val webSocketScheme: String,
    val livenessPath: String,
    val readinessPath: String,
    val credential: BackendConnectionCredential,
)
