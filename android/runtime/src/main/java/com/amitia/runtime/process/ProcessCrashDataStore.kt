package com.amitia.runtime.process

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.intPreferencesKey
import androidx.datastore.preferences.core.longPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import java.util.concurrent.ConcurrentHashMap
import javax.inject.Inject
import javax.inject.Singleton

private const val PROCESS_CRASH_DATASTORE_NAME = "amitia_process_crash"

private val Context.processCrashDataStore: DataStore<Preferences> by preferencesDataStore(
    name = PROCESS_CRASH_DATASTORE_NAME
)

data class ProcessCrashData(
    val crashCount: Int = 0,
    val lastExitReason: String? = null,
    val lastExitCode: Int? = null,
    val lastStartedAt: Long? = null
)

@Singleton
class ProcessCrashDataStore @Inject constructor(
    @ApplicationContext private val context: Context
) {

    private val dataStore: DataStore<Preferences> = context.processCrashDataStore

    private val cache = ConcurrentHashMap<String, ProcessCrashData>()

    fun cached(name: String): ProcessCrashData =
        cache[name] ?: ProcessCrashData()

    fun cachedCrashCount(name: String): Int = cached(name).crashCount

    fun cachedLastExitReason(name: String): String? = cached(name).lastExitReason

    fun cachedLastExitCode(name: String): Int? = cached(name).lastExitCode

    fun cachedLastStartedAt(name: String): Long? = cached(name).lastStartedAt

    suspend fun load(name: String): ProcessCrashData {
        cache[name]?.let { return it }
        val data = dataStore.data.map { prefs ->
            ProcessCrashData(
                crashCount = prefs[crashCountKey(name)] ?: 0,
                lastExitReason = prefs[lastExitReasonKey(name)],
                lastExitCode = prefs[lastExitCodeKey(name)],
                lastStartedAt = prefs[lastStartedAtKey(name)]
            )
        }.first()
        cache[name] = data
        return data
    }

    suspend fun save(name: String, data: ProcessCrashData) {
        cache[name] = data
        dataStore.edit { prefs ->
            prefs[crashCountKey(name)] = data.crashCount
            if (data.lastExitReason != null) {
                prefs[lastExitReasonKey(name)] = data.lastExitReason
            } else {
                prefs.remove(lastExitReasonKey(name))
            }
            if (data.lastExitCode != null) {
                prefs[lastExitCodeKey(name)] = data.lastExitCode
            } else {
                prefs.remove(lastExitCodeKey(name))
            }
            if (data.lastStartedAt != null) {
                prefs[lastStartedAtKey(name)] = data.lastStartedAt
            } else {
                prefs.remove(lastStartedAtKey(name))
            }
        }
    }

    suspend fun clear(name: String) {
        cache.remove(name)
        dataStore.edit { prefs ->
            prefs.remove(crashCountKey(name))
            prefs.remove(lastExitReasonKey(name))
            prefs.remove(lastExitCodeKey(name))
            prefs.remove(lastStartedAtKey(name))
        }
    }

    suspend fun clearAll() {
        cache.clear()
        dataStore.edit { it.clear() }
    }

    private fun crashCountKey(name: String) = intPreferencesKey("crash_count_$name")

    private fun lastExitReasonKey(name: String) = stringPreferencesKey("last_exit_reason_$name")

    private fun lastExitCodeKey(name: String) = intPreferencesKey("last_exit_code_$name")

    private fun lastStartedAtKey(name: String) = longPreferencesKey("last_started_at_$name")
}
