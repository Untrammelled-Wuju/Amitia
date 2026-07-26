package com.amitia.core.datastore

import androidx.test.core.app.ApplicationProvider
import com.amitia.core.datastore.SettingsDataStore.RunMode
import com.amitia.core.datastore.SettingsDataStore.RuntimeStrategy
import com.amitia.core.datastore.SettingsDataStore.ThemeMode
import com.google.common.truth.Truth.assertThat
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.flow.first
import org.junit.After
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], manifest = Config.NONE)
class SettingsDataStoreTest {

    private lateinit var store: SettingsDataStore

    @Before
    fun setUp() {
        val context = ApplicationProvider.getApplicationContext<android.content.Context>()
        store = SettingsDataStore(context)
        runBlocking {
            store.setRuntimeVersion(null)
            store.setRuntimeStrategy(RuntimeStrategy.ON_DEMAND)
            store.setRunMode(RunMode.LOCAL)
            store.setThemeMode(ThemeMode.SYSTEM)
            store.setOnboardingCompleted(false)
            store.setOnboardingStep(0)
        }
    }

    @After
    fun tearDown() {
        runBlocking {
            store.setRuntimeVersion(null)
            store.setRuntimeStrategy(RuntimeStrategy.ON_DEMAND)
            store.setRunMode(RunMode.LOCAL)
            store.setThemeMode(ThemeMode.SYSTEM)
        }
    }

    @Test
    fun default_runtime_version_is_null() = runBlocking {
        val version = store.runtimeVersion.first()

        assertThat(version).isNull()
    }

    @Test
    fun setRuntimeVersion_persists_and_reads_back() = runBlocking {
        store.setRuntimeVersion("1.2.3")

        val version = store.runtimeVersion.first()
        assertThat(version).isEqualTo("1.2.3")
    }

    @Test
    fun setRuntimeVersion_with_null_clears_value() = runBlocking {
        store.setRuntimeVersion("1.0.0")
        store.setRuntimeVersion(null)

        assertThat(store.runtimeVersion.first()).isNull()
    }

    @Test
    fun runtime_version_migration_from_legacy_to_new_version() = runBlocking {
        store.setRuntimeVersion("0.9.0")

        val legacy = store.runtimeVersion.first()
        assertThat(legacy).isEqualTo("0.9.0")

        store.setRuntimeVersion("1.0.0")

        val migrated = store.runtimeVersion.first()
        assertThat(migrated).isEqualTo("1.0.0")
        assertThat(migrated).isNotEqualTo(legacy)
    }

    @Test
    fun runtime_version_migration_preserves_strategy_across_version_change() = runBlocking {
        store.setRuntimeVersion("0.9.0")
        store.setRuntimeStrategy(RuntimeStrategy.ALWAYS_ON)

        store.setRuntimeVersion("1.0.0")

        assertThat(store.runtimeStrategy.first()).isEqualTo(RuntimeStrategy.ALWAYS_ON)
    }

    @Test
    fun default_runtime_strategy_is_ON_DEMAND() = runBlocking {
        val strategy = store.runtimeStrategy.first()

        assertThat(strategy).isEqualTo(RuntimeStrategy.ON_DEMAND)
    }

    @Test
    fun setRuntimeStrategy_persists_and_reads_back() = runBlocking {
        store.setRuntimeStrategy(RuntimeStrategy.ALWAYS_ON)

        assertThat(store.runtimeStrategy.first()).isEqualTo(RuntimeStrategy.ALWAYS_ON)
    }

    @Test
    fun setRuntimeStrategy_supports_REMOTE_ONLY_migration() = runBlocking {
        store.setRuntimeStrategy(RuntimeStrategy.REMOTE_ONLY)

        assertThat(store.runtimeStrategy.first()).isEqualTo(RuntimeStrategy.REMOTE_ONLY)
        assertThat(store.currentRuntimeStrategy()).isEqualTo(RuntimeStrategy.REMOTE_ONLY)
    }

    @Test
    fun setRunMode_persists_and_reads_back() = runBlocking {
        store.setRunMode(RunMode.REMOTE)

        assertThat(store.runMode.first()).isEqualTo(RunMode.REMOTE)
        assertThat(store.currentRunMode()).isEqualTo(RunMode.REMOTE)
    }

    @Test
    fun setThemeMode_persists_and_reads_back() = runBlocking {
        store.setThemeMode(ThemeMode.DARK)

        assertThat(store.themeMode.first()).isEqualTo(ThemeMode.DARK)
        assertThat(store.currentThemeMode()).isEqualTo(ThemeMode.DARK)
    }

    @Test
    fun setRemoteBaseUrl_persists_and_strips_blank() = runBlocking {
        store.setRemoteBaseUrl("https://api.example.com")

        assertThat(store.remoteBaseUrl.first()).isEqualTo("https://api.example.com")

        store.setRemoteBaseUrl("")
        assertThat(store.remoteBaseUrl.first()).isEqualTo("")
    }

    @Test
    fun setOnboardingCompleted_persists_and_reads_back() = runBlocking {
        store.setOnboardingCompleted(true)

        assertThat(store.onboardingCompleted.first()).isTrue()
        assertThat(store.isOnboardingCompleted()).isTrue()
    }

    @Test
    fun setOnboardingStep_advances_through_flow() = runBlocking {
        store.setOnboardingStep(1)
        assertThat(store.onboardingStep.first()).isEqualTo(1)

        store.setOnboardingStep(7)
        assertThat(store.onboardingStep.first()).isEqualTo(7)
    }

    @Test
    fun setCacheDirHint_persists_and_clears_blank() = runBlocking {
        store.setCacheDirHint("/data/cache")

        assertThat(store.cacheDirHint.first()).isEqualTo("/data/cache")

        store.setCacheDirHint("")
        assertThat(store.cacheDirHint.first()).isEqualTo("")
    }
}
