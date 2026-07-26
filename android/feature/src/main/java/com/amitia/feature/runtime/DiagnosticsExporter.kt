package com.amitia.feature.runtime

import java.io.File
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

object DiagnosticsExporter {

    fun export(
        directory: File,
        state: RuntimeUiState,
        logs: List<LogEntry>
    ): File {
        if (!directory.exists()) directory.mkdirs()
        val timestamp = SimpleDateFormat("yyyyMMdd_HHmmss", Locale.getDefault()).format(Date())
        val file = File(directory, "amitia_diagnostics_$timestamp.txt")
        val builder = StringBuilder()
        builder.appendLine("Amitia Diagnostics Export")
        builder.appendLine("Generated: ${Date()}")
        builder.appendLine()
        builder.appendLine("=== Runtime State ===")
        builder.appendLine("Phase: ${state.runtimeState.phase}")
        builder.appendLine("Message: ${state.runtimeState.readableMessage}")
        builder.appendLine("Progress: ${state.runtimeState.progress}")
        builder.appendLine("UptimeMs: ${state.uptimeMs}")
        builder.appendLine("RootfsVersion: ${state.rootfsVersion ?: "unknown"}")
        builder.appendLine("LastError: ${state.lastError ?: "none"}")
        builder.appendLine()
        builder.appendLine("=== Services ===")
        builder.appendLine("Backend: ${formatService(state.services.backend)}")
        builder.appendLine("Qdrant: ${formatService(state.services.qdrant)}")
        builder.appendLine("SurrealDB: ${formatService(state.services.surrealDb)}")
        builder.appendLine()
        builder.appendLine("=== Ports ===")
        builder.appendLine("Backend: ${state.ports.backend ?: "-"}")
        builder.appendLine("Qdrant: ${state.ports.qdrant ?: "-"}")
        builder.appendLine("SurrealDB: ${state.ports.surrealdb ?: "-"}")
        builder.appendLine()
        builder.appendLine("=== Data Usage ===")
        builder.appendLine("amitia-data: ${formatBytes(state.dataUsage.dataBytes)}")
        builder.appendLine("rootfs: ${formatBytes(state.dataUsage.rootfsBytes)}")
        builder.appendLine()
        builder.appendLine("=== Logs (${logs.size} entries) ===")
        val timeFormat = SimpleDateFormat("yyyy-MM-dd HH:mm:ss.SSS", Locale.getDefault())
        logs.forEach { entry ->
            val time = timeFormat.format(Date(entry.timestampMs))
            builder.appendLine("[$time] [${entry.level}] [${entry.tag}] ${entry.message}")
        }
        file.writeText(builder.toString())
        return file
    }

    private fun formatService(state: com.amitia.runtime.api.ServiceState): String {
        return when (state) {
            is com.amitia.runtime.api.ServiceState.Healthy -> "Healthy@${state.port}"
            is com.amitia.runtime.api.ServiceState.Unhealthy -> "Unhealthy(${state.reason})"
            com.amitia.runtime.api.ServiceState.Stopped -> "Stopped"
            com.amitia.runtime.api.ServiceState.Starting -> "Starting"
        }
    }

    private fun formatBytes(bytes: Long): String {
        if (bytes <= 0) return "0 B"
        val units = arrayOf("B", "KB", "MB", "GB", "TB")
        var size = bytes.toDouble()
        var unitIndex = 0
        while (size >= 1024 && unitIndex < units.lastIndex) {
            size /= 1024
            unitIndex++
        }
        return "%.2f %s".format(size, units[unitIndex])
    }
}
