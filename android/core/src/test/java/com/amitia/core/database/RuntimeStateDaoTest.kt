package com.amitia.core.database

import androidx.room.Room
import androidx.test.core.app.ApplicationProvider
import com.amitia.core.database.dao.RuntimeStateDao
import com.amitia.core.database.entity.RuntimeStateEntity
import com.google.common.truth.Truth.assertThat
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.runBlocking
import org.junit.After
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], manifest = Config.NONE)
class RuntimeStateDaoTest {

    private lateinit var database: AmitiaDatabase
    private lateinit var dao: RuntimeStateDao

    @Before
    fun setUp() {
        val context = ApplicationProvider.getApplicationContext<android.content.Context>()
        database = Room.inMemoryDatabaseBuilder(context, AmitiaDatabase::class.java)
            .allowMainThreadQueries()
            .build()
        dao = database.runtimeStateDao()
    }

    @After
    fun tearDown() {
        database.close()
    }

    @Test
    fun get_returns_null_when_no_state_persisted() = runBlocking {
        val state = dao.get()

        assertThat(state).isNull()
    }

    @Test
    fun upsert_inserts_new_state_with_singleton_id() = runBlocking {
        val entity = RuntimeStateEntity(
            id = 1,
            state = "RUNNING",
            services = "{}",
            updatedAt = 1000L,
            snapshot = null
        )

        dao.upsert(entity)

        val stored = dao.get()
        assertThat(stored).isNotNull()
        assertThat(stored!!.state).isEqualTo("RUNNING")
        assertThat(stored.updatedAt).isEqualTo(1000L)
    }

    @Test
    fun upsert_replaces_existing_state_keeps_singleton_id() = runBlocking {
        dao.upsert(RuntimeStateEntity(id = 1, state = "STARTING", updatedAt = 1L))

        dao.upsert(RuntimeStateEntity(id = 1, state = "RUNNING", updatedAt = 2L))

        val stored = dao.get()
        assertThat(stored).isNotNull()
        assertThat(stored!!.state).isEqualTo("RUNNING")
        assertThat(stored.updatedAt).isEqualTo(2L)
    }

    @Test
    fun clear_removes_singleton_state() = runBlocking {
        dao.upsert(RuntimeStateEntity(id = 1, state = "STOPPED", updatedAt = 5L))

        dao.clear()

        assertThat(dao.get()).isNull()
    }

    @Test
    fun data_directory_strategy_isolated_per_database_instance() = runBlocking {
        val firstState = dao.get()
        assertThat(firstState).isNull()

        dao.upsert(RuntimeStateEntity(id = 1, state = "INSTALLED", updatedAt = 10L))

        val context = ApplicationProvider.getApplicationContext<android.content.Context>()
        val anotherDb = Room.inMemoryDatabaseBuilder(context, AmitiaDatabase::class.java)
            .allowMainThreadQueries()
            .build()

        val anotherDao = anotherDb.runtimeStateDao()
        assertThat(anotherDao.get()).isNull()

        anotherDb.close()
    }

    @Test
    fun snapshot_field_round_trips() = runBlocking {
        val snapshot = """{"services":{"backend":"HEALTHY","qdrant":"HEALTHY"}}"""
        dao.upsert(
            RuntimeStateEntity(
                id = 1,
                state = "RUNNING",
                services = snapshot,
                updatedAt = 99L,
                snapshot = snapshot
            )
        )

        val stored = dao.get()
        assertThat(stored).isNotNull()
        assertThat(stored!!.snapshot).isEqualTo(snapshot)
        assertThat(stored.services).isEqualTo(snapshot)
    }

    @Test
    fun observe_emits_inserted_state_then_null_after_clear() = runBlocking {
        dao.upsert(RuntimeStateEntity(id = 1, state = "RUNNING", updatedAt = 1L))

        val first = dao.observe().first()
        assertThat(first).isNotNull()
        assertThat(first!!.state).isEqualTo("RUNNING")

        dao.clear()
        val second = dao.observe().first()
        assertThat(second).isNull()
    }
}
