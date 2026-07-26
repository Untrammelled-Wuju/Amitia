package com.amitia.feature.chat.media

import com.amitia.core.repository.UploadRepository
import java.io.File
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class ImageUploader @Inject constructor(
    private val uploadRepository: UploadRepository,
    private val imageCompressor: ImageCompressor
) {

    suspend fun upload(
        file: File,
        onProgress: (Float) -> Unit = {}
    ): Result<String> {
        return runCatching {
            val compressed = imageCompressor.compress(file)
            onProgress(0.05f)
            val response = uploadRepository.uploadImage(
                file = compressed,
                onProgress = { sent, total ->
                    if (total > 0) onProgress((sent.toFloat() / total.toFloat()).coerceIn(0f, 1f))
                }
            )
            onProgress(1f)
            response.url
        }
    }

    suspend fun uploadMany(
        files: List<File>,
        onProgress: (overall: Float, currentIndex: Int, total: Int) -> Unit = { _, _, _ -> }
    ): Result<List<String>> {
        val urls = mutableListOf<String>()
        files.forEachIndexed { index, file ->
            onProgress(index.toFloat() / files.size.toFloat(), index, files.size)
            val result = upload(file)
            if (result.isSuccess) {
                urls.add(result.getOrThrow())
            } else {
                return Result.failure(result.exceptionOrNull() ?: IllegalStateException("上传失败"))
            }
        }
        onProgress(1f, files.size, files.size)
        return Result.success(urls)
    }
}
