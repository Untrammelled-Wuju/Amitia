package com.amitia.amitia_app.workflow

import android.content.ContentProvider
import android.content.ContentValues
import android.database.Cursor
import android.net.Uri
import com.amitia.amitia_app.nativeprovider.AndroidNativeCompositionRoot
import com.amitia.amitia_app.runtime.recovery.PersistentRuntimeRecoveryScheduler

class WorkflowStartupProvider : ContentProvider() {
    override fun onCreate(): Boolean {
        val app = context?.applicationContext ?: return false
        AndroidNativeCompositionRoot.initialize(app)
        WorkflowSystemEventRegistrar.ensureRegistered(app)
        WorkflowAutomationHealthJobService.schedule(app)
        PersistentRuntimeRecoveryScheduler.ensureScheduledFromStore(app)
        return true
    }

    override fun query(uri: Uri, projection: Array<out String>?, selection: String?, selectionArgs: Array<out String>?, sortOrder: String?): Cursor? = null
    override fun getType(uri: Uri): String? = null
    override fun insert(uri: Uri, values: ContentValues?): Uri? = null
    override fun delete(uri: Uri, selection: String?, selectionArgs: Array<out String>?): Int = 0
    override fun update(uri: Uri, values: ContentValues?, selection: String?, selectionArgs: Array<out String>?): Int = 0
}
