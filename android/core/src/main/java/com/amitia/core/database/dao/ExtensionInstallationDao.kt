package com.amitia.core.database.dao

import androidx.room.Dao
import androidx.room.Delete
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.Query
import androidx.room.Update
import com.amitia.core.database.entity.ExtensionInstallationEntity
import kotlinx.coroutines.flow.Flow

@Dao
interface ExtensionInstallationDao {

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insert(entity: ExtensionInstallationEntity): Long

    @Update
    suspend fun update(entity: ExtensionInstallationEntity)

    @Delete
    suspend fun delete(entity: ExtensionInstallationEntity)

    @Query("DELETE FROM extension_installations WHERE extensionId = :extensionId")
    suspend fun deleteByExtensionId(extensionId: String)

    @Query("SELECT * FROM extension_installations WHERE id = :id")
    suspend fun getById(id: Long): ExtensionInstallationEntity?

    @Query("SELECT * FROM extension_installations WHERE extensionId = :extensionId LIMIT 1")
    suspend fun getByExtensionId(extensionId: String): ExtensionInstallationEntity?

    @Query("SELECT * FROM extension_installations ORDER BY installedAt DESC")
    suspend fun getAll(): List<ExtensionInstallationEntity>

    @Query("SELECT * FROM extension_installations ORDER BY installedAt DESC")
    fun observeAll(): Flow<List<ExtensionInstallationEntity>>

    @Query("UPDATE extension_installations SET status = :status WHERE extensionId = :extensionId")
    suspend fun updateStatus(extensionId: String, status: String)

    @Query("SELECT COUNT(*) FROM extension_installations")
    suspend fun count(): Int
}
