package com.amitia.feature.chat.tts

import com.amitia.core.model.TtsRequest
import com.amitia.core.repository.TtsRepository
import com.amitia.feature.chat.audio.AudioPlayerController
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch

@Singleton
class TtsController @Inject constructor(
    private val ttsRepository: TtsRepository,
    private val playerController: AudioPlayerController,
    private val preferences: TtsPreferences
) {

    private val scope = CoroutineScope(SupervisorJob())

    fun play(audioUrl: String) {
        playerController.play(audioUrl, scope)
    }

    suspend fun synthesize(
        text: String,
        voice: String?,
        characterId: String? = null
    ): Result<String> {
        if (text.isBlank()) return Result.failure(IllegalArgumentException("文本为空"))
        val effectiveVoice = voice ?: preferences.currentVoice()
        val request = TtsRequest(
            text = text,
            voiceId = effectiveVoice,
            characterId = characterId
        )
        val primary = runCatching { ttsRepository.synthesize(request) }
        if (primary.isSuccess) {
            return Result.success(primary.getOrThrow().audioUrl)
        }
        val fallbackRequest = request.copy(voiceId = "default")
        val fallback = runCatching { ttsRepository.synthesize(fallbackRequest) }
        if (fallback.isSuccess) {
            return Result.success(fallback.getOrThrow().audioUrl)
        }
        return Result.failure(primary.exceptionOrNull() ?: IllegalStateException("TTS 不可用"))
    }

    fun autoPlayIfNeeded(text: String, voice: String?, characterId: String?) {
        scope.launch {
            val enabled = preferences.currentAutoPlay()
            if (!enabled) return@launch
            val result = synthesize(text, voice, characterId)
            result.onSuccess { url -> play(url) }
        }
    }

    suspend fun setAutoPlay(enabled: Boolean) {
        preferences.setAutoPlay(enabled)
    }

    suspend fun setPreferredVoice(voiceId: String?) {
        preferences.setPreferredVoice(voiceId)
    }
}
