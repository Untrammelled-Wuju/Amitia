package com.amitia.core.network.connection

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.longPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first

private const val SESSION_DATASTORE_NAME = "amitia_session"

private val Context.sessionDataStore: DataStore<Preferences> by preferencesDataStore(
    name = SESSION_DATASTORE_NAME
)

@Singleton
class SessionManager @Inject constructor(
    @ApplicationContext private val context: Context
) {

    private val dataStore: DataStore<Preferences> = context.sessionDataStore

    private val sessionState = MutableStateFlow<Session?>(null)

    val session: StateFlow<Session?> = sessionState.asStateFlow()

    suspend fun loadInitial() {
        val prefs = dataStore.data.first()
        val token = prefs[KEY_TOKEN] ?: return
        val expiresAt = prefs[KEY_EXPIRES_AT] ?: 0L
        val userId = prefs[KEY_USER_ID]
        sessionState.value = Session(
            token = token,
            expiresAt = expiresAt,
            userId = userId
        )
    }

    suspend fun saveSession(token: String, expiresAt: Long, userId: String?) {
        dataStore.edit { prefs ->
            prefs[KEY_TOKEN] = token
            prefs[KEY_EXPIRES_AT] = expiresAt
            if (userId != null) {
                prefs[KEY_USER_ID] = userId
            } else {
                prefs.remove(KEY_USER_ID)
            }
        }
        sessionState.value = Session(
            token = token,
            expiresAt = expiresAt,
            userId = userId
        )
    }

    suspend fun clearSession() {
        dataStore.edit { prefs ->
            prefs.remove(KEY_TOKEN)
            prefs.remove(KEY_EXPIRES_AT)
            prefs.remove(KEY_USER_ID)
        }
        sessionState.value = null
    }

    fun isExpired(): Boolean {
        val current = sessionState.value ?: return true
        if (current.expiresAt <= 0L) return false
        return System.currentTimeMillis() >= current.expiresAt
    }

    fun current(): Session? = sessionState.value

    data class Session(
        val token: String,
        val expiresAt: Long,
        val userId: String?
    )

    companion object {
        private val KEY_TOKEN = stringPreferencesKey("token")
        private val KEY_EXPIRES_AT = longPreferencesKey("expires_at")
        private val KEY_USER_ID = stringPreferencesKey("user_id")
    }
}
