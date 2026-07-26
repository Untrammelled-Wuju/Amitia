package com.amitia.feature.onboarding

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map

private const val ONBOARDING_DATASTORE_NAME = "amitia_onboarding"

private val Context.onboardingDataStore: DataStore<Preferences> by preferencesDataStore(
    name = ONBOARDING_DATASTORE_NAME
)

@Singleton
class OnboardingDataStore @Inject constructor(
    @ApplicationContext private val context: Context
) {

    private val dataStore: DataStore<Preferences> = context.onboardingDataStore

    val currentStepName: Flow<String?> = dataStore.data.map { it[KEY_STEP] }

    val remoteConfig: Flow<RemoteConfigSnapshot> = dataStore.data.map { prefs ->
        RemoteConfigSnapshot(
            baseUrl = prefs[KEY_REMOTE_BASE_URL].orEmpty(),
            authToken = prefs[KEY_REMOTE_TOKEN].orEmpty(),
            mode = prefs[KEY_MODE] ?: "local"
        )
    }

    suspend fun saveStep(stepName: String) {
        dataStore.edit { it[KEY_STEP] = stepName }
    }

    suspend fun saveRemoteConfig(baseUrl: String, token: String, mode: String) {
        dataStore.edit { prefs ->
            prefs[KEY_REMOTE_BASE_URL] = baseUrl
            prefs[KEY_REMOTE_TOKEN] = token
            prefs[KEY_MODE] = mode
        }
    }

    suspend fun saveModelConfigSnapshot(snapshot: ModelConfigSnapshot) {
        dataStore.edit { prefs ->
            prefs[KEY_MODEL_PROVIDER] = snapshot.provider
            prefs[KEY_MODEL_NAME] = snapshot.modelName
            prefs[KEY_MODEL_API_KEY] = snapshot.apiKey
            prefs[KEY_MODEL_ENDPOINT] = snapshot.endpoint
        }
    }

    suspend fun loadModelConfigSnapshot(): ModelConfigSnapshot? {
        val prefs = dataStore.data.first()
        val provider = prefs[KEY_MODEL_PROVIDER] ?: return null
        return ModelConfigSnapshot(
            provider = provider,
            modelName = prefs[KEY_MODEL_NAME].orEmpty(),
            apiKey = prefs[KEY_MODEL_API_KEY].orEmpty(),
            endpoint = prefs[KEY_MODEL_ENDPOINT].orEmpty()
        )
    }

    suspend fun clear() {
        dataStore.edit { it.clear() }
    }

    data class RemoteConfigSnapshot(
        val baseUrl: String,
        val authToken: String,
        val mode: String
    )

    data class ModelConfigSnapshot(
        val provider: String,
        val modelName: String,
        val apiKey: String,
        val endpoint: String
    )

    companion object {
        private val KEY_STEP = stringPreferencesKey("step")
        private val KEY_MODE = stringPreferencesKey("mode")
        private val KEY_REMOTE_BASE_URL = stringPreferencesKey("remote_base_url")
        private val KEY_REMOTE_TOKEN = stringPreferencesKey("remote_token")
        private val KEY_MODEL_PROVIDER = stringPreferencesKey("model_provider")
        private val KEY_MODEL_NAME = stringPreferencesKey("model_name")
        private val KEY_MODEL_API_KEY = stringPreferencesKey("model_api_key")
        private val KEY_MODEL_ENDPOINT = stringPreferencesKey("model_endpoint")
    }
}
