package com.amitia.core.database.dao

import androidx.room.Dao
import androidx.room.Delete
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.Query
import androidx.room.Update
import com.amitia.core.database.entity.ProactiveMessageEntity
import kotlinx.coroutines.flow.Flow

@Dao
interface ProactiveDao {

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsert(entity: ProactiveMessageEntity)

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsertAll(entities: List<ProactiveMessageEntity>)

    @Update
    suspend fun update(entity: ProactiveMessageEntity)

    @Delete
    suspend fun delete(entity: ProactiveMessageEntity)

    @Query("DELETE FROM proactive_messages WHERE id = :id")
    suspend fun deleteById(id: String)

    @Query("SELECT * FROM proactive_messages WHERE id = :id")
    suspend fun getById(id: String): ProactiveMessageEntity?

    @Query("SELECT * FROM proactive_messages ORDER BY createdAt DESC")
    fun observeAll(): Flow<List<ProactiveMessageEntity>>

    @Query("SELECT * FROM proactive_messages WHERE characterId = :characterId ORDER BY createdAt DESC")
    fun observeByCharacter(characterId: String): Flow<List<ProactiveMessageEntity>>

    @Query("SELECT * FROM proactive_messages WHERE isRead = 0 ORDER BY createdAt ASC")
    suspend fun listUnread(): List<ProactiveMessageEntity>

    @Query("SELECT * FROM proactive_messages WHERE isNotified = 0 AND isRead = 0 ORDER BY createdAt ASC")
    suspend fun listUnnotified(): List<ProactiveMessageEntity>

    @Query("UPDATE proactive_messages SET isRead = 1 WHERE id = :id")
    suspend fun markRead(id: String)

    @Query("UPDATE proactive_messages SET isNotified = 1 WHERE id = :id")
    suspend fun markNotified(id: String)

    @Query("UPDATE proactive_messages SET isRead = 1 WHERE id IN (:ids)")
    suspend fun markReadAll(ids: List<String>)

    @Query("SELECT COUNT(*) FROM proactive_messages WHERE isRead = 0")
    fun observeUnreadCount(): Flow<Int>
}
