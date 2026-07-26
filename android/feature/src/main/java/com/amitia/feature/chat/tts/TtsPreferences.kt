package com.amitia.feature.chat.tts

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map

private const val TTS_DATASTORE_NAME = "amitia_tts"

private val Context.ttsDataStore: DataStore<Preferences> by preferencesDataStore(
    name = TTS_DATASTORE_NAME
)

@Singleton
class TtsPreferences @Inject constructor(
    @ApplicationContext private val context: Context
) {

    private val dataStore: DataStore<Preferences> = context.ttsDataStore

    val autoPlay: Flow<Boolean> = dataStore.data.map { it[KEY_AUTO_PLAY] ?: false }

    val preferredVoice: Flow<String?> = dataStore.data.map { it[KEY_VOICE_ID] }

    suspend fun setAutoPlay(enabled: Boolean) {
        dataStore.edit { it[KEY_AUTO_PLAY] = enabled }
    }

    suspend fun setPreferredVoice(voiceId: String?) {
        dataStore.edit { prefs ->
            if (voiceId.isNullOrBlank()) prefs.remove(KEY_VOICE_ID)
            else prefs[KEY_VOICE_ID] = voiceId
        }
    }

    suspend fun currentAutoPlay(): Boolean = autoPlay.first()
    suspend fun currentVoice(): String? = preferredVoice.first()

    companion object {
        private val KEY_AUTO_PLAY = booleanPreferencesKey("auto_play")
        private val KEY_VOICE_ID = stringPreferencesKey("voice_id")
    }
}
