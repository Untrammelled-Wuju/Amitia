package com.amitia.amitia_app.runtime.connection.internal

import com.amitia.amitia_app.runtime.connection.BackendConnectionCredential
import com.amitia.amitia_app.runtime.connection.BackendConnectionError
import com.amitia.amitia_app.runtime.connection.BackendConnectionErrorCode
import java.io.File

internal class RuntimeCredentialResolver {
    fun resolve(dataRootPath: String): Result<BackendConnectionCredential> {
        val dataRoot = File(dataRootPath)
        val tokenFile = File(dataRoot, "security/local-token")

        val resolvedTokenFile = try { tokenFile.canonicalFile } catch (_: Throwable) { return Result.failure(BackendConnectionError(BackendConnectionErrorCode.CREDENTIAL_INVALID, "failed to resolve token path")) }
        val resolvedDataRoot = try { dataRoot.canonicalFile } catch (_: Throwable) { return Result.failure(BackendConnectionError(BackendConnectionErrorCode.CREDENTIAL_INVALID, "failed to resolve data root")) }

        if (!resolvedTokenFile.path.startsWith(resolvedDataRoot.path + File.separator)) {
            return Result.failure(
                BackendConnectionError(BackendConnectionErrorCode.CREDENTIAL_INVALID, "token path escapes data root")
            )
        }

        if (!tokenFile.exists()) {
            return Result.failure(
                BackendConnectionError(BackendConnectionErrorCode.CREDENTIAL_UNAVAILABLE, "local token file not found")
            )
        }
        if (!tokenFile.isFile) {
            return Result.failure(
                BackendConnectionError(BackendConnectionErrorCode.CREDENTIAL_UNAVAILABLE, "token path is not a regular file")
            )
        }
        val maxSize = 16 * 1024L
        if (tokenFile.length() > maxSize) {
            return Result.failure(
                BackendConnectionError(BackendConnectionErrorCode.CREDENTIAL_INVALID, "token file exceeds max size")
            )
        }

        return runCatching {
            val raw = tokenFile.readText(Charsets.UTF_8).trim()
            BackendConnectionCredential.create(raw)
        }.fold(
            onSuccess = { Result.success(it) },
            onFailure = { err ->
                Result.failure(
                    BackendConnectionError(BackendConnectionErrorCode.CREDENTIAL_INVALID, err.message ?: "invalid credential")
                )
            }
        )
    }
}
