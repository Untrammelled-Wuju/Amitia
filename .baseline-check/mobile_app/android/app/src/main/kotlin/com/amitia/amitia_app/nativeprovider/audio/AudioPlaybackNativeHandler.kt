package com.amitia.amitia_app.nativeprovider.audio

import android.media.AudioAttributes
import android.media.MediaPlayer
import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.File

internal class AudioPlaybackNativeHandler : AndroidNativeOperationHandler {
    @Volatile
    private var player: MediaPlayer? = null

    override val operations: Set<String> = setOf(OP_PLAY_FILE, OP_STOP)

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse {
        return when (request.operation) {
            OP_PLAY_FILE -> playFile(request)
            OP_STOP -> stop(request)
            else -> error(request, NativeBridgeProtocol.ERR_OPERATION_NOT_SUPPORTED, "unknown audio operation: ${request.operation}")
        }
    }

    private suspend fun playFile(request: NativeBridgeRequest): NativeBridgeResponse = withContext(Dispatchers.IO) {
        val path = request.payload["path"]?.toString()?.trim().orEmpty()
        if (path.isEmpty()) {
            return@withContext error(request, NativeBridgeProtocol.ERR_INVALID_REQUEST, "audio file path is required")
        }
        val file = File(path)
        if (!file.isFile || !file.canRead()) {
            return@withContext error(request, "AUDIO_FILE_UNAVAILABLE", "audio file is unavailable: $path")
        }
        try {
            releasePlayer()
            val mediaPlayer = MediaPlayer().apply {
                setAudioAttributes(
                    AudioAttributes.Builder()
                        .setContentType(AudioAttributes.CONTENT_TYPE_SPEECH)
                        .setUsage(AudioAttributes.USAGE_MEDIA)
                        .build(),
                )
                setDataSource(file.absolutePath)
                setOnCompletionListener { completed ->
                    synchronized(this@AudioPlaybackNativeHandler) {
                        if (player === completed) player = null
                    }
                    completed.release()
                }
                setOnErrorListener { failed, _, _ ->
                    synchronized(this@AudioPlaybackNativeHandler) {
                        if (player === failed) player = null
                    }
                    failed.release()
                    true
                }
                prepare()
            }
            synchronized(this@AudioPlaybackNativeHandler) {
                player = mediaPlayer
            }
            mediaPlayer.start()
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_SUCCESS,
                result = mapOf(
                    "playing" to true,
                    "path" to file.absolutePath,
                    "durationMs" to mediaPlayer.duration,
                ),
            )
        } catch (e: Exception) {
            releasePlayer()
            error(request, "AUDIO_PLAYBACK_FAILED", e.message ?: "audio playback failed")
        }
    }

    private fun stop(request: NativeBridgeRequest): NativeBridgeResponse {
        releasePlayer()
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf("playing" to false),
        )
    }

    @Synchronized
    private fun releasePlayer() {
        val current = player
        player = null
        if (current != null) {
            try {
                if (current.isPlaying) current.stop()
            } catch (_: Exception) {
            }
            try {
                current.reset()
            } catch (_: Exception) {
            }
            try {
                current.release()
            } catch (_: Exception) {
            }
        }
    }

    private fun error(request: NativeBridgeRequest, code: String, message: String): NativeBridgeResponse {
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_ERROR,
            error = NativeBridgeError(code = code, message = message),
        )
    }

    companion object {
        const val OP_PLAY_FILE = "media.audio.play_file"
        const val OP_STOP = "media.audio.stop"
    }
}
