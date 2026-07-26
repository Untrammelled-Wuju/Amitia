package com.amitia.feature.chat.media

import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Matrix
import java.io.ByteArrayOutputStream
import java.io.File
import java.io.FileOutputStream
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class ImageCompressor @Inject constructor() {

    fun compress(file: File, maxEdge: Int = MAX_EDGE, quality: Int = QUALITY): File {
        if (!file.exists()) return file
        val original = decodeBitmap(file) ?: return file
        val scaled = scaleIfNeeded(original, maxEdge)
        val out = ByteArrayOutputStream()
        scaled.compress(Bitmap.CompressFormat.JPEG, quality, out)
        val compressed = File(file.parentFile, "compressed_${System.currentTimeMillis()}.jpg")
        FileOutputStream(compressed).use { it.write(out.toByteArray()) }
        if (scaled != original) original.recycle()
        return compressed
    }

    fun rotate(file: File, degrees: Int): File {
        if (degrees == 0) return file
        val bitmap = decodeBitmap(file) ?: return file
        val matrix = Matrix().apply { postRotate(degrees.toFloat()) }
        val rotated = Bitmap.createBitmap(bitmap, 0, 0, bitmap.width, bitmap.height, matrix, true)
        val out = ByteArrayOutputStream()
        rotated.compress(Bitmap.CompressFormat.JPEG, QUALITY, out)
        val target = File(file.parentFile, "rotated_${System.currentTimeMillis()}.jpg")
        FileOutputStream(target).use { it.write(out.toByteArray()) }
        if (rotated != bitmap) bitmap.recycle()
        return target
    }

    private fun decodeBitmap(file: File): Bitmap? {
        val options = BitmapFactory.Options().apply { inPreferredConfig = Bitmap.Config.ARGB_8888 }
        return BitmapFactory.decodeFile(file.absolutePath, options)
    }

    private fun scaleIfNeeded(bitmap: Bitmap, maxEdge: Int): Bitmap {
        val longest = maxOf(bitmap.width, bitmap.height)
        if (longest <= maxEdge) return bitmap
        val scale = maxEdge.toFloat() / longest.toFloat()
        val width = (bitmap.width * scale).toInt()
        val height = (bitmap.height * scale).toInt()
        return Bitmap.createScaledBitmap(bitmap, width, height, true)
    }

    companion object {
        const val MAX_EDGE = 1920
        const val QUALITY = 85
        const val MAX_IMAGES = 9
        val ALLOWED_MIME_TYPES = setOf("image/jpeg", "image/png", "image/webp")
    }
}
