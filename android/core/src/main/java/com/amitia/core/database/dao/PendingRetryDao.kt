package com.amitia.core.database.dao

import androidx.room.Dao
import androidx.room.Delete
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.Query
import androidx.room.Update
import com.amitia.core.database.entity.PendingRetryEntity
import kotlinx.coroutines.flow.Flow

@Dao
interface PendingRetryDao {

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsert(entity: PendingRetryEntity)

    @Update
    suspend fun update(entity: PendingRetryEntity)

    @Delete
    suspend fun delete(entity: PendingRetryEntity)

    @Query("DELETE FROM pending_retries WHERE id = :id")
    suspend fun deleteById(id: String)

    @Query("SELECT * FROM pending_retries WHERE id = :id")
    suspend fun getById(id: String): PendingRetryEntity?

    @Query("SELECT * FROM pending_retries ORDER BY nextRetryAt ASC")
    fun observeAll(): Flow<List<PendingRetryEntity>>

    @Query("SELECT * FROM pending_retries WHERE nextRetryAt <= :now ORDER BY nextRetryAt ASC")
    suspend fun listDue(now: Long): List<PendingRetryEntity>

    @Query("UPDATE pending_retries SET retryCount = retryCount + 1, nextRetryAt = :nextAt WHERE id = :id")
    suspend fun incrementRetry(id: String, nextAt: Long)

    @Query("SELECT COUNT(*) FROM pending_retries")
    suspend fun count(): Int
}
