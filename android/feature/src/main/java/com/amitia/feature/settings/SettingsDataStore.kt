package com.amitia.feature.settings

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

private const val SETTINGS_DATASTORE_NAME = "amitia_settings"

private val Context.settingsDataStore: DataStore<Preferences> by preferencesDataStore(
    name = SETTINGS_DATASTORE_NAME
)

@Singleton
class SettingsDataStore @Inject constructor(
    @ApplicationContext private val context: Context
) {

    private val dataStore: DataStore<Preferences> = context.settingsDataStore

    val themeMode: Flow<ThemeMode> = dataStore.data.map { prefs ->
        runCatching { ThemeMode.valueOf(prefs[KEY_THEME] ?: ThemeMode.SYSTEM.name) }
            .getOrDefault(ThemeMode.SYSTEM)
    }

    val notificationsEnabled: Flow<Boolean> = dataStore.data.map { it[KEY_NOTIFICATIONS] ?: true }
    val ttsAutoPlay: Flow<Boolean> = dataStore.data.map { it[KEY_TTS_AUTO] ?: false }
    val voicePreferred: Flow<String?> = dataStore.data.map { it[KEY_VOICE_ID] }
    val remoteBaseUrl: Flow<String> = dataStore.data.map { it[KEY_REMOTE_URL].orEmpty() }
    val cacheDirHint: Flow<String> = dataStore.data.map { it[KEY_CACHE_HINT].orEmpty() }
    val logLevel: Flow<String> = dataStore.data.map { it[KEY_LOG_LEVEL] ?: "info" }

    suspend fun setThemeMode(mode: ThemeMode) {
        dataStore.edit { it[KEY_THEME] = mode.name }
    }

    suspend fun setNotificationsEnabled(enabled: Boolean) {
        dataStore.edit { it[KEY_NOTIFICATIONS] = enabled }
    }

    suspend fun setTtsAutoPlay(enabled: Boolean) {
        dataStore.edit { it[KEY_TTS_AUTO] = enabled }
    }

    suspend fun setPreferredVoice(voiceId: String?) {
        dataStore.edit { prefs ->
            if (voiceId.isNullOrBlank()) prefs.remove(KEY_VOICE_ID)
            else prefs[KEY_VOICE_ID] = voiceId
        }
    }

    suspend fun setRemoteBaseUrl(url: String) {
        dataStore.edit { prefs ->
            if (url.isBlank()) prefs.remove(KEY_REMOTE_URL)
            else prefs[KEY_REMOTE_URL] = url
        }
    }

    suspend fun setCacheDirHint(path: String) {
        dataStore.edit { prefs ->
            if (path.isBlank()) prefs.remove(KEY_CACHE_HINT)
            else prefs[KEY_CACHE_HINT] = path
        }
    }

    suspend fun setLogLevel(level: String) {
        dataStore.edit { it[KEY_LOG_LEVEL] = level }
    }

    suspend fun currentThemeMode(): ThemeMode = themeMode.first()

    enum class ThemeMode { SYSTEM, DARK, LIGHT }

    companion object {
        private val KEY_THEME = stringPreferencesKey("theme_mode")
        private val KEY_NOTIFICATIONS = booleanPreferencesKey("notifications_enabled")
        private val KEY_TTS_AUTO = booleanPreferencesKey("tts_auto_play")
        private val KEY_VOICE_ID = stringPreferencesKey("preferred_voice")
        private val KEY_REMOTE_URL = stringPreferencesKey("remote_url")
        private val KEY_CACHE_HINT = stringPreferencesKey("cache_dir_hint")
        private val KEY_LOG_LEVEL = stringPreferencesKey("log_level")
    }
}
