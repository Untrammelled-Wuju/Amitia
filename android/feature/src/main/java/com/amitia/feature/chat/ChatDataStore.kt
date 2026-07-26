package com.amitia.feature.chat

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

private const val CHAT_DATASTORE_NAME = "amitia_chat"

private val Context.chatDataStore: DataStore<Preferences> by preferencesDataStore(
    name = CHAT_DATASTORE_NAME
)

@Singleton
class ChatDataStore @Inject constructor(
    @ApplicationContext private val context: Context
) {

    private val dataStore: DataStore<Preferences> = context.chatDataStore

    fun observeDraft(characterId: String): Flow<String> = dataStore.data.map { prefs ->
        prefs[draftKey(characterId)].orEmpty()
    }

    suspend fun loadDraft(characterId: String): String {
        return dataStore.data.first()[draftKey(characterId)].orEmpty()
    }

    suspend fun saveDraft(characterId: String, draft: String) {
        dataStore.edit { prefs ->
            if (draft.isBlank()) prefs.remove(draftKey(characterId))
            else prefs[draftKey(characterId)] = draft
        }
    }

    suspend fun clearDraft(characterId: String) {
        dataStore.edit { it.remove(draftKey(characterId)) }
    }

    private fun draftKey(characterId: String) = stringPreferencesKey("draft_$characterId")
}
