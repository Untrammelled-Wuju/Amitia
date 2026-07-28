package com.amitia.core.repository

import com.amitia.core.model.UploadResponse
import java.io.File
import java.util.UUID
import javax.inject.Inject
import javax.inject.Singleton
import okio.buffer
import okio.sink

@Singleton
class UploadRepository @Inject constructor() {

    suspend fun uploadImage(
        file: File,
        mimeType: String = "image/jpeg",
        onProgress: ((sent: Long, total: Long) -> Unit)? = null
    ): UploadResponse {
        onProgress?.invoke(file.length(), file.length())
        return UploadResponse(
            url = "https://mock.example.com/uploads/${UUID.randomUUID()}.jpg",
            filename = file.name,
            mimeType = mimeType,
            size = file.length()
        )
    }

    suspend fun uploadAudio(
        file: File,
        mimeType: String = "audio/wav",
        onProgress: ((sent: Long, total: Long) -> Unit)? = null
    ): UploadResponse {
        onProgress?.invoke(file.length(), file.length())
        return UploadResponse(
            url = "https://mock.example.com/uploads/${UUID.randomUUID()}.aac",
            filename = file.name,
            mimeType = mimeType,
            size = file.length()
        )
    }

    fun buildTempFile(dir: File, name: String, bytes: ByteArray): File {
        if (!dir.exists()) dir.mkdirs()
        val file = File(dir, name)
        file.sink().buffer().use { sink ->
            sink.write(bytes)
        }
        return file
    }

    fun mapFailure(throwable: Throwable): com.amitia.core.network.client.AmitiaApiException {
        return com.amitia.core.network.client.AmitiaApiException.UploadFailed(throwable.message)
    }
}
