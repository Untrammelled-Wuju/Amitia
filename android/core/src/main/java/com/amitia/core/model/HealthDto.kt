package com.amitia.core.model

import kotlinx.serialization.Serializable

@Serializable
data class HealthResponse(
    val status: String? = null,
    val version: String? = null,
    val uptime: Long? = null,
    val services: Map<String, ServiceStatus>? = null
)

@Serializable
data class ServiceStatus(
    val name: String,
    val healthy: Boolean,
    val port: Int? = null,
    val reason: String? = null
)
