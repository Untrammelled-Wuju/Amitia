package com.amitia.runtime.extension

import android.content.Context
import androidx.work.CoroutineWorker
import androidx.work.WorkerParameters
import dagger.hilt.EntryPoint
import dagger.hilt.InstallIn
import dagger.hilt.android.EntryPointAccessors
import dagger.hilt.components.SingletonComponent
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive

class ExtensionScheduleWorker(
    context: Context,
    params: WorkerParameters
) : CoroutineWorker(context, params) {

    override suspend fun doWork(): Result {
        val entryPoint = EntryPointAccessors.fromApplication(
            applicationContext,
            ExtensionWorkerEntryPoint::class.java
        )
        val apiClient = entryPoint.extensionApiClient()

        return runCatching {
            val schedulesResponse = apiClient.listSchedules()
            val items = schedulesResponse["items"]?.jsonArray ?: JsonArray(emptyList())

            items.forEach { item ->
                val obj = item.jsonObject
                val definition = obj["definition"]?.jsonObject
                val executionOwner = definition?.get("executionOwner")?.jsonPrimitive?.contentOrNull
                val state = obj["state"]?.jsonObject
                val status = state?.get("status")?.jsonPrimitive?.contentOrNull
                val scheduleId = state?.get("scheduleId")?.jsonPrimitive?.contentOrNull
                    ?: definition?.get("scheduleId")?.jsonPrimitive?.contentOrNull

                if (scheduleId != null && status == "enabled" && executionOwner == "android") {
                    runCatching { apiClient.runScheduleNow(scheduleId, "android_worker") }
                }
            }

            Result.success()
        }.getOrElse { e ->
            Result.retry()
        }
    }

    @EntryPoint
    @InstallIn(SingletonComponent::class)
    interface ExtensionWorkerEntryPoint {
        fun extensionApiClient(): ExtensionApiClient
    }

    companion object {
        const val WORK_NAME = "extension_schedule_worker"
    }
}
