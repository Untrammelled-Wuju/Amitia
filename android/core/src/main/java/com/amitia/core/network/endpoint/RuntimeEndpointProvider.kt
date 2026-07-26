package com.amitia.core.network.endpoint

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import com.amitia.core.common.Constants
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first

private const val ENDPOINT_DATASTORE_NAME = "amitia_endpoint"

private val Context.endpointDataStore: DataStore<Preferences> by preferencesDataStore(
    name = ENDPOINT_DATASTORE_NAME
)

@Singleton
class RuntimeEndpointProvider @Inject constructor(
    @ApplicationContext private val context: Context
) {

    private val dataStore: DataStore<Preferences> = context.endpointDataStore

    private val endpointState = MutableStateFlow<RuntimeEndpoint>(
        RuntimeEndpoint.Local(
            host = Constants.LOCAL_HOST,
            port = Constants.BACKEND_PORT,
            authToken = null
        )
    )

    val currentEndpoint: StateFlow<RuntimeEndpoint> = endpointState.asStateFlow()

    suspend fun loadInitial() {
        val mode = getCurrentMode()
        val endpoint = when (mode) {
            RuntimeMode.LOCAL -> RuntimeEndpoint.Local(
                host = Constants.LOCAL_HOST,
                port = Constants.BACKEND_PORT,
                authToken = null
            )
            RuntimeMode.REMOTE -> {
                val baseUrl = dataStore.data.first()[KEY_REMOTE_BASE_URL].orEmpty()
                RuntimeEndpoint.Remote(baseUrl = baseUrl, authToken = null)
            }
        }
        endpointState.value = endpoint
    }

    suspend fun switchToLocal(authToken: String?) {
        dataStore.edit { prefs ->
            prefs[KEY_MODE] = RuntimeMode.LOCAL.name
            if (authToken != null) {
                prefs[KEY_AUTH_TOKEN] = authToken
            } else {
                prefs.remove(KEY_AUTH_TOKEN)
            }
            prefs.remove(KEY_REMOTE_BASE_URL)
        }
        endpointState.value = RuntimeEndpoint.Local(
            host = Constants.LOCAL_HOST,
            port = Constants.BACKEND_PORT,
            authToken = authToken
        )
    }

    suspend fun switchToRemote(baseUrl: String, authToken: String?) {
        val normalizedBaseUrl = baseUrl.removeSuffix("/")
        dataStore.edit { prefs ->
            prefs[KEY_MODE] = RuntimeMode.REMOTE.name
            prefs[KEY_REMOTE_BASE_URL] = normalizedBaseUrl
            if (authToken != null) {
                prefs[KEY_AUTH_TOKEN] = authToken
            } else {
                prefs.remove(KEY_AUTH_TOKEN)
            }
        }
        endpointState.value = RuntimeEndpoint.Remote(
            baseUrl = normalizedBaseUrl,
            authToken = authToken
        )
    }

    suspend fun getCurrentMode(): RuntimeMode {
        val name = dataStore.data.first()[KEY_MODE] ?: RuntimeMode.LOCAL.name
        return runCatching { RuntimeMode.valueOf(name) }.getOrDefault(RuntimeMode.LOCAL)
    }

    suspend fun getStoredAuthToken(): String? {
        return dataStore.data.first()[KEY_AUTH_TOKEN]
    }

    fun observeEndpoint(): Flow<RuntimeEndpoint> = endpointState.asStateFlow()

    enum class RuntimeMode { LOCAL, REMOTE }

    companion object {
        private val KEY_MODE = stringPreferencesKey("mode")
        private val KEY_REMOTE_BASE_URL = stringPreferencesKey("remote_base_url")
        private val KEY_AUTH_TOKEN = stringPreferencesKey("auth_token")
    }
}
