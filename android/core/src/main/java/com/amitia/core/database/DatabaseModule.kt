package com.amitia.core.database

import android.content.Context
import androidx.room.Room
import com.amitia.core.database.dao.CharacterDao
import com.amitia.core.database.dao.ConversationDao
import com.amitia.core.database.dao.DraftDao
import com.amitia.core.database.dao.MessageDao
import com.amitia.core.database.dao.PendingRetryDao
import com.amitia.core.database.dao.ProactiveDao
import com.amitia.core.database.dao.RuntimeStateDao
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.android.qualifiers.ApplicationContext
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton

@Module
@InstallIn(SingletonComponent::class)
object DatabaseModule {

    @Provides
    @Singleton
    fun provideDatabase(@ApplicationContext context: Context): AmitiaDatabase {
        return Room.databaseBuilder(
            context,
            AmitiaDatabase::class.java,
            AmitiaDatabase.DATABASE_NAME
        )
            .fallbackToDestructiveMigration()
            .build()
    }

    @Provides
    fun provideCharacterDao(db: AmitiaDatabase): CharacterDao = db.characterDao()

    @Provides
    fun provideConversationDao(db: AmitiaDatabase): ConversationDao = db.conversationDao()

    @Provides
    fun provideMessageDao(db: AmitiaDatabase): MessageDao = db.messageDao()

    @Provides
    fun provideDraftDao(db: AmitiaDatabase): DraftDao = db.draftDao()

    @Provides
    fun provideProactiveDao(db: AmitiaDatabase): ProactiveDao = db.proactiveDao()

    @Provides
    fun provideRuntimeStateDao(db: AmitiaDatabase): RuntimeStateDao = db.runtimeStateDao()

    @Provides
    fun providePendingRetryDao(db: AmitiaDatabase): PendingRetryDao = db.pendingRetryDao()
}
