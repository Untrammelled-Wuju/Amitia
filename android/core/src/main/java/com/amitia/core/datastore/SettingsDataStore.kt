package com.amitia.core.datastore

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.intPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map

private const val APP_SETTINGS_DATASTORE_NAME = "amitia_app_settings"

private val Context.appSettingsDataStore: DataStore<Preferences> by preferencesDataStore(
    name = APP_SETTINGS_DATASTORE_NAME
)

@Singleton
class SettingsDataStore @Inject constructor(
    @ApplicationContext private val context: Context
) {

    private val dataStore: DataStore<Preferences> = context.appSettingsDataStore

    val runMode: Flow<RunMode> = dataStore.data.map { prefs ->
        runCatching { RunMode.valueOf(prefs[KEY_RUN_MODE] ?: RunMode.LOCAL.name) }
            .getOrDefault(RunMode.LOCAL)
    }

    val remoteBaseUrl: Flow<String> = dataStore.data.map { it[KEY_REMOTE_BASE_URL].orEmpty() }

    val currentCharacterId: Flow<String?> = dataStore.data.map { it[KEY_CURRENT_CHARACTER_ID] }

    val themeMode: Flow<ThemeMode> = dataStore.data.map { prefs ->
        runCatching { ThemeMode.valueOf(prefs[KEY_THEME_MODE] ?: ThemeMode.SYSTEM.name) }
            .getOrDefault(ThemeMode.SYSTEM)
    }

    val notificationEnabled: Flow<Boolean> = dataStore.data.map { it[KEY_NOTIFICATION_ENABLED] ?: true }

    val notificationPrivacy: Flow<NotificationPrivacy> = dataStore.data.map { prefs ->
        runCatching { NotificationPrivacy.valueOf(prefs[KEY_NOTIFICATION_PRIVACY] ?: NotificationPrivacy.CONTENT.name) }
            .getOrDefault(NotificationPrivacy.CONTENT)
    }

    val voiceAutoPlay: Flow<Boolean> = dataStore.data.map { it[KEY_VOICE_AUTO_PLAY] ?: false }

    val voicePreferred: Flow<String?> = dataStore.data.map { it[KEY_VOICE_PREFERRED] }

    val onboardingCompleted: Flow<Boolean> = dataStore.data.map { it[KEY_ONBOARDING_COMPLETED] ?: false }

    val onboardingStep: Flow<Int> = dataStore.data.map { it[KEY_ONBOARDING_STEP] ?: 0 }

    val runtimeVersion: Flow<String?> = dataStore.data.map { it[KEY_RUNTIME_VERSION] }

    val runtimeStrategy: Flow<RuntimeStrategy> = dataStore.data.map { prefs ->
        runCatching { RuntimeStrategy.valueOf(prefs[KEY_RUNTIME_STRATEGY] ?: RuntimeStrategy.ON_DEMAND.name) }
            .getOrDefault(RuntimeStrategy.ON_DEMAND)
    }

    val logLevel: Flow<String> = dataStore.data.map { it[KEY_LOG_LEVEL] ?: "info" }

    val cacheDirHint: Flow<String> = dataStore.data.map { it[KEY_CACHE_DIR_HINT].orEmpty() }

    suspend fun setRunMode(mode: RunMode) {
        dataStore.edit { it[KEY_RUN_MODE] = mode.name }
    }

    suspend fun setRemoteBaseUrl(url: String) {
        dataStore.edit { prefs ->
            if (url.isBlank()) prefs.remove(KEY_REMOTE_BASE_URL)
            else prefs[KEY_REMOTE_BASE_URL] = url
        }
    }

    suspend fun setCurrentCharacterId(characterId: String?) {
        dataStore.edit { prefs ->
            if (characterId.isNullOrBlank()) prefs.remove(KEY_CURRENT_CHARACTER_ID)
            else prefs[KEY_CURRENT_CHARACTER_ID] = characterId
        }
    }

    suspend fun setThemeMode(mode: ThemeMode) {
        dataStore.edit { it[KEY_THEME_MODE] = mode.name }
    }

    suspend fun setNotificationEnabled(enabled: Boolean) {
        dataStore.edit { it[KEY_NOTIFICATION_ENABLED] = enabled }
    }

    suspend fun setNotificationPrivacy(privacy: NotificationPrivacy) {
        dataStore.edit { it[KEY_NOTIFICATION_PRIVACY] = privacy.name }
    }

    suspend fun setVoiceAutoPlay(enabled: Boolean) {
        dataStore.edit { it[KEY_VOICE_AUTO_PLAY] = enabled }
    }

    suspend fun setVoicePreferred(voiceId: String?) {
        dataStore.edit { prefs ->
            if (voiceId.isNullOrBlank()) prefs.remove(KEY_VOICE_PREFERRED)
            else prefs[KEY_VOICE_PREFERRED] = voiceId
        }
    }

    suspend fun setOnboardingCompleted(completed: Boolean) {
        dataStore.edit { it[KEY_ONBOARDING_COMPLETED] = completed }
    }

    suspend fun setOnboardingStep(step: Int) {
        dataStore.edit { it[KEY_ONBOARDING_STEP] = step }
    }

    suspend fun setRuntimeVersion(version: String?) {
        dataStore.edit { prefs ->
            if (version.isNullOrBlank()) prefs.remove(KEY_RUNTIME_VERSION)
            else prefs[KEY_RUNTIME_VERSION] = version
        }
    }

    suspend fun setRuntimeStrategy(strategy: RuntimeStrategy) {
        dataStore.edit { it[KEY_RUNTIME_STRATEGY] = strategy.name }
    }

    suspend fun setLogLevel(level: String) {
        dataStore.edit { it[KEY_LOG_LEVEL] = level }
    }

    suspend fun setCacheDirHint(path: String) {
        dataStore.edit { prefs ->
            if (path.isBlank()) prefs.remove(KEY_CACHE_DIR_HINT)
            else prefs[KEY_CACHE_DIR_HINT] = path
        }
    }

    fun characterNotificationEnabled(characterId: String): Flow<Boolean> =
        dataStore.data.map { prefs ->
            val key = KEY_CHARACTER_NOTIFICATION_PREFIX + characterId
            prefs[booleanPreferencesKey(key)] ?: true
        }

    suspend fun setCharacterNotificationEnabled(characterId: String, enabled: Boolean) {
        dataStore.edit { prefs ->
            val key = KEY_CHARACTER_NOTIFICATION_PREFIX + characterId
            prefs[booleanPreferencesKey(key)] = enabled
        }
    }

    suspend fun characterNotificationEnabledNow(characterId: String): Boolean =
        characterNotificationEnabled(characterId).first()

    suspend fun currentRunMode(): RunMode = runMode.first()

    suspend fun currentThemeMode(): ThemeMode = themeMode.first()

    suspend fun currentNotificationPrivacy(): NotificationPrivacy = notificationPrivacy.first()

    suspend fun currentRuntimeStrategy(): RuntimeStrategy = runtimeStrategy.first()

    suspend fun isOnboardingCompleted(): Boolean = onboardingCompleted.first()

    enum class RunMode { LOCAL, REMOTE }

    enum class ThemeMode { SYSTEM, DARK, LIGHT }

    enum class NotificationPrivacy { CONTENT, HIDDEN, ANNOUNCEMENT_ONLY }

    enum class RuntimeStrategy { ALWAYS_ON, ON_DEMAND, REMOTE_ONLY }

    companion object {
        private val KEY_RUN_MODE = stringPreferencesKey("run_mode")
        private val KEY_REMOTE_BASE_URL = stringPreferencesKey("remote_base_url")
        private val KEY_CURRENT_CHARACTER_ID = stringPreferencesKey("current_character_id")
        private val KEY_THEME_MODE = stringPreferencesKey("theme_mode")
        private val KEY_NOTIFICATION_ENABLED = booleanPreferencesKey("notification_enabled")
        private val KEY_NOTIFICATION_PRIVACY = stringPreferencesKey("notification_privacy")
        private val KEY_VOICE_AUTO_PLAY = booleanPreferencesKey("voice_auto_play")
        private val KEY_VOICE_PREFERRED = stringPreferencesKey("voice_preferred")
        private val KEY_ONBOARDING_COMPLETED = booleanPreferencesKey("onboarding_completed")
        private val KEY_ONBOARDING_STEP = intPreferencesKey("onboarding_step")
        private val KEY_RUNTIME_VERSION = stringPreferencesKey("runtime_version")
        private val KEY_RUNTIME_STRATEGY = stringPreferencesKey("runtime_strategy")
        private val KEY_LOG_LEVEL = stringPreferencesKey("log_level")
        private val KEY_CACHE_DIR_HINT = stringPreferencesKey("cache_dir_hint")
        const val KEY_CHARACTER_NOTIFICATION_PREFIX = "character_notification_"
    }
}
