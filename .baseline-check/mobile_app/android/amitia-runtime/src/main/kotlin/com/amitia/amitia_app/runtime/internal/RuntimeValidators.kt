package com.amitia.amitia_app.runtime.internal

import com.amitia.amitia_app.runtime.api.RuntimeProgress

internal object RuntimeValidators {

    private val COMPONENT_ID_REGEX = Regex("^[a-z0-9][a-z0-9._\\-]{2,127}$")
    private const val MAX_PACKAGE_URI_LENGTH = 4096
    private const val MAX_OPERATION_ID_LENGTH = 256
    private const val MAX_DETAILS_COUNT = 32
    private const val MAX_DETAIL_KEY_LENGTH = 128
    private const val MAX_DETAIL_VALUE_LENGTH = 1024

    fun isValidComponentId(id: String): Boolean {
        if (id.length < 3 || id.length > 128) return false
        return COMPONENT_ID_REGEX.matches(id)
    }

    fun isNullOrInvalidComponentId(id: String?): Boolean {
        if (id == null) return false
        return !isValidComponentId(id)
    }

    fun isValidOperationId(operationId: String): Boolean {
        if (operationId.isEmpty()) return false
        if (operationId.length > MAX_OPERATION_ID_LENGTH) return false
        if (operationId.contains("/")) return false
        if (operationId.contains("\\")) return false
        if (operationId.contains(":")) return false
        return true
    }

    fun isValidPackageUri(uri: String): Boolean {
        if (uri.isEmpty()) return false
        if (uri.length > MAX_PACKAGE_URI_LENGTH) return false
        return true
    }

    fun isValidErrorDetails(details: Map<String, String>): Boolean {
        if (details.size > MAX_DETAILS_COUNT) return false
        for ((key, value) in details) {
            if (key.length > MAX_DETAIL_KEY_LENGTH) return false
            if (value.length > MAX_DETAIL_VALUE_LENGTH) return false
            if (containsControlCharacter(key)) return false
            if (containsControlCharacter(value)) return false
        }
        return true
    }

    fun isValidProgress(progress: RuntimeProgress): Boolean {
        if (progress.completedUnits < 0L) return false
        if (progress.totalUnits < 0L) return false
        if (progress.totalUnits == 0L) {
            return progress.percent == 0
        }
        return progress.percent in 0..100
    }

    private fun containsControlCharacter(value: String): Boolean {
        for (ch in value) {
            if (ch == '\n' || ch == '\r' || ch == '\t') return true
            if (ch.code in 0x00..0x1F && ch != '\t') return true
        }
        return false
    }
}
