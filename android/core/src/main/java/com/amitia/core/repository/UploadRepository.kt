package com.amitia.core.repository

import com.amitia.core.model.UploadResponse
import com.amitia.core.network.api.UploadApi
import com.amitia.core.network.client.AmitiaApiClient
import com.amitia.core.network.client.AmitiaApiException
import java.io.File
import javax.inject.Inject
import javax.inject.Singleton
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.MultipartBody
import okhttp3.RequestBody.Companion.asRequestBody
import okio.buffer
import okio.sink

@Singleton
class UploadRepository @Inject constructor(
    private val apiClient: AmitiaApiClient
) {

    private val api: UploadApi by lazy { apiClient.service(UploadApi::class.java) }

    suspend fun uploadImage(
        file: File,
        mimeType: String = DEFAULT_IMAGE_MIME,
        onProgress: ((sent: Long, total: Long) -> Unit)? = null
    ): UploadResponse {
        return upload(file, mimeType, type = "image", onProgress = onProgress)
    }

    suspend fun uploadAudio(
        file: File,
        mimeType: String = DEFAULT_AUDIO_MIME,
        onProgress: ((sent: Long, total: Long) -> Unit)? = null
    ): UploadResponse {
        return runCatching {
            upload(file, mimeType, type = "audio", onProgress = onProgress)
        }.getOrElse {
            uploadAsrAudio(file, mimeType, onProgress)
        }
    }

    private suspend fun upload(
        file: File,
        mimeType: String,
        type: String,
        onProgress: ((sent: Long, total: Long) -> Unit)?
    ): UploadResponse {
        onProgress?.invoke(0L, file.length())
        val mediaType = mimeType.toMediaType()
        val requestBody = file.asRequestBody(mediaType)
        val multipart = MultipartBody.Part.createFormData(
            FIELD_FILE,
            file.name,
            requestBody
        )
        val response = api.upload(multipart, type)
        onProgress?.invoke(file.length(), file.length())
        return response
    }

    private suspend fun uploadAsrAudio(
        file: File,
        mimeType: String,
        onProgress: ((sent: Long, total: Long) -> Unit)?
    ): UploadResponse {
        onProgress?.invoke(0L, file.length())
        val mediaType = mimeType.toMediaType()
        val requestBody = file.asRequestBody(mediaType)
        val multipart = MultipartBody.Part.createFormData(
            FIELD_FILE,
            file.name,
            requestBody
        )
        val response = api.uploadAudio(multipart)
        onProgress?.invoke(file.length(), file.length())
        return response
    }

    fun buildTempFile(dir: File, name: String, bytes: ByteArray): File {
        if (!dir.exists()) dir.mkdirs()
        val file = File(dir, name)
        file.sink().buffer().use { sink ->
            sink.write(bytes)
        }
        return file
    }

    fun mapFailure(throwable: Throwable): AmitiaApiException {
        return AmitiaApiException.UploadFailed(throwable.message)
    }

    companion object {
        private const val FIELD_FILE = "file"
        private const val DEFAULT_IMAGE_MIME = "image/jpeg"
        private const val DEFAULT_AUDIO_MIME = "audio/wav"
    }
}
