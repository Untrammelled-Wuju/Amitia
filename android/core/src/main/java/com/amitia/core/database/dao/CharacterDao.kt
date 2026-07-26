package com.amitia.core.database.dao

import androidx.room.Dao
import androidx.room.Delete
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.Query
import androidx.room.Update
import com.amitia.core.database.entity.CharacterEntity
import kotlinx.coroutines.flow.Flow

@Dao
interface CharacterDao {

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsert(entity: CharacterEntity)

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsertAll(entities: List<CharacterEntity>)

    @Update
    suspend fun update(entity: CharacterEntity)

    @Delete
    suspend fun delete(entity: CharacterEntity)

    @Query("DELETE FROM characters WHERE id = :id")
    suspend fun deleteById(id: String)

    @Query("SELECT * FROM characters WHERE id = :id")
    suspend fun getById(id: String): CharacterEntity?

    @Query("SELECT * FROM characters ORDER BY updatedAt DESC")
    fun observeAll(): Flow<List<CharacterEntity>>

    @Query("SELECT * FROM characters WHERE isCurrent = 1 LIMIT 1")
    fun observeCurrent(): Flow<CharacterEntity?>

    @Query("UPDATE characters SET isCurrent = (id = :id)")
    suspend fun setCurrent(id: String)

    @Query("SELECT COUNT(*) FROM characters")
    suspend fun count(): Int
}
