package com.amitia.core.database.dao

import androidx.room.Dao
import androidx.room.Delete
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.Query
import androidx.room.Update
import com.amitia.core.database.entity.DraftEntity
import kotlinx.coroutines.flow.Flow

@Dao
interface DraftDao {

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsert(entity: DraftEntity)

    @Update
    suspend fun update(entity: DraftEntity)

    @Delete
    suspend fun delete(entity: DraftEntity)

    @Query("SELECT * FROM drafts WHERE characterId = :characterId")
    fun observeByCharacter(characterId: String): Flow<DraftEntity?>

    @Query("SELECT * FROM drafts WHERE characterId = :characterId")
    suspend fun getByCharacter(characterId: String): DraftEntity?

    @Query("DELETE FROM drafts WHERE characterId = :characterId")
    suspend fun deleteByCharacter(characterId: String)

    @Query("DELETE FROM drafts")
    suspend fun clearAll()
}
