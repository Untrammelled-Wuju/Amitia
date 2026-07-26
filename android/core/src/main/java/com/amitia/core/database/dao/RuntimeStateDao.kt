package com.amitia.core.database.dao

import androidx.room.Dao
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.Query
import com.amitia.core.database.entity.RuntimeStateEntity
import kotlinx.coroutines.flow.Flow

@Dao
interface RuntimeStateDao {

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsert(entity: RuntimeStateEntity)

    @Query("SELECT * FROM runtime_state WHERE id = 1")
    suspend fun get(): RuntimeStateEntity?

    @Query("SELECT * FROM runtime_state WHERE id = 1")
    fun observe(): Flow<RuntimeStateEntity?>

    @Query("DELETE FROM runtime_state WHERE id = 1")
    suspend fun clear()
}
