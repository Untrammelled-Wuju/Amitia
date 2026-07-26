package com.amitia.runtime.health

data class ServiceHealth(
    val serviceName: String,
    val port: Int,
    val isPortOpen: Boolean = false,
    val isHttpHealthy: Boolean = false,
    val isProcessAlive: Boolean = false,
    val lastCheckedAt: Long = System.currentTimeMillis(),
    val errorMessage: String? = null
) {

    fun isHealthy(): Boolean = isPortOpen && isHttpHealthy && isProcessAlive

    fun isDegraded(): Boolean = !isHealthy() && (isPortOpen || isProcessAlive)

    fun isDown(): Boolean = !isPortOpen && !isProcessAlive

    fun toServiceState(): com.amitia.runtime.api.ServiceState = when {
        isHealthy() -> com.amitia.runtime.api.ServiceState.Healthy(port)
        isDown() -> com.amitia.runtime.api.ServiceState.Stopped
        else -> com.amitia.runtime.api.ServiceState.Unhealthy(errorMessage ?: "服务降级")
    }
}
