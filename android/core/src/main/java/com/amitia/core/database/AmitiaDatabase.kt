package com.amitia.core.database

import androidx.room.Database
import androidx.room.RoomDatabase
import androidx.room.TypeConverters
import com.amitia.core.database.converter.Converters
import com.amitia.core.database.dao.CharacterDao
import com.amitia.core.database.dao.ConversationDao
import com.amitia.core.database.dao.DraftDao
import com.amitia.core.database.dao.MessageDao
import com.amitia.core.database.dao.PendingRetryDao
import com.amitia.core.database.dao.ProactiveDao
import com.amitia.core.database.dao.RuntimeStateDao
import com.amitia.core.database.entity.CharacterEntity
import com.amitia.core.database.entity.ConversationEntity
import com.amitia.core.database.entity.DraftEntity
import com.amitia.core.database.entity.MessageEntity
import com.amitia.core.database.entity.PendingRetryEntity
import com.amitia.core.database.entity.ProactiveMessageEntity
import com.amitia.core.database.entity.RuntimeStateEntity

@Database(
    entities = [
        CharacterEntity::class,
        ConversationEntity::class,
        MessageEntity::class,
        DraftEntity::class,
        ProactiveMessageEntity::class,
        RuntimeStateEntity::class,
        PendingRetryEntity::class
    ],
    version = 1,
    exportSchema = false
)
@TypeConverters(Converters::class)
abstract class AmitiaDatabase : RoomDatabase() {

    abstract fun characterDao(): CharacterDao

    abstract fun conversationDao(): ConversationDao

    abstract fun messageDao(): MessageDao

    abstract fun draftDao(): DraftDao

    abstract fun proactiveDao(): ProactiveDao

    abstract fun runtimeStateDao(): RuntimeStateDao

    abstract fun pendingRetryDao(): PendingRetryDao

    companion object {
        const val DATABASE_NAME = "amitia.db"
    }
}
