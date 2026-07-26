package com.amitia.platform.bridge.provider

import com.amitia.platform.audio.AudioRecorder
import com.amitia.platform.bridge.CapabilityProvider
import com.amitia.platform.bridge.NativeActionRequest
import com.amitia.platform.bridge.NativeActionResult
import com.amitia.platform.permissions.PermissionBroker
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class MicRecordProvider @Inject constructor(
    private val audioRecorder: AudioRecorder,
    private val permissionBroker: PermissionBroker
) : CapabilityProvider {

    override fun action(): String = "mic_record"

    override fun requiredPermission(): String? = PermissionBroker.Permissions.RECORD_AUDIO

    override suspend fun execute(request: NativeActionRequest): NativeActionResult {
        val action = request.params["op"] ?: "status"
        return when (action) {
            "status" -> NativeActionResult.Success(mapOf("state" to audioRecorder.state::class.simpleName.orEmpty()))
            "prepare" -> {
                val outputParam = request.params["output"]
                val outputFile = if (!outputParam.isNullOrBlank()) outputParam
                else "${request.params["cache_dir"] ?: System.getProperty("java.io.tmpdir")}/voice_${System.currentTimeMillis()}.aac"
                val sampleRate = request.params["sample_rate"]?.toIntOrNull() ?: 44_100
                val bitrate = request.params["bitrate"]?.toIntOrNull() ?: 128_000
                val channels = request.params["channels"]?.toIntOrNull() ?: 1
                val encoding = request.params["encoding"]?.let { mapEncoding(it) }
                    ?: AudioRecorder.AudioEncoding.AAC
                val config = AudioRecorder.RecordingConfig(
                    outputFile = outputFile,
                    sampleRate = sampleRate,
                    channels = channels,
                    encoding = encoding,
                    bitrate = bitrate
                )
                val ok = audioRecorder.prepare(config)
                if (ok) NativeActionResult.Success(mapOf("state" to "prepared", "output" to outputFile))
                else NativeActionResult.Failed("prepare_failed")
            }
            "start" -> {
                audioRecorder.start()
                NativeActionResult.Success(mapOf("state" to "recording"))
            }
            "pause" -> {
                audioRecorder.pause()
                NativeActionResult.Success(mapOf("state" to "paused"))
            }
            "resume" -> {
                audioRecorder.resume()
                NativeActionResult.Success(mapOf("state" to "recording"))
            }
            "amplitude" -> {
                val amp = audioRecorder.getMaxAmplitude()
                NativeActionResult.Success(mapOf("amplitude" to amp.toString()))
            }
            "stop" -> {
                val result = audioRecorder.stop()
                if (result.success) {
                    NativeActionResult.Success(
                        mapOf(
                            "path" to (result.filePath ?: ""),
                            "duration" to result.durationMillis.toString(),
                            "size" to result.sizeBytes.toString()
                        )
                    )
                } else {
                    NativeActionResult.Failed(result.error ?: "stop_failed")
                }
            }
            "cancel" -> {
                audioRecorder.cancel()
                NativeActionResult.Success(mapOf("state" to "cancelled"))
            }
            else -> NativeActionResult.Failed("unsupported op: $action")
        }
    }

    private fun mapEncoding(value: String): AudioRecorder.AudioEncoding? {
        return runCatching { AudioRecorder.AudioEncoding.valueOf(value.uppercase()) }.getOrNull()
    }
}
