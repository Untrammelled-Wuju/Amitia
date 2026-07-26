package com.amitia.platform.bridge

import android.content.Context
import android.content.Intent
import android.net.Uri
import androidx.activity.ComponentActivity
import androidx.activity.result.ActivityResultLauncher
import androidx.activity.result.contract.ActivityResultContracts
import androidx.core.content.FileProvider
import com.amitia.core.logging.Logger
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import java.io.File
import javax.inject.Inject
import javax.inject.Singleton
import kotlin.coroutines.resume

interface ActivityResultBridge {

    suspend fun requestPermission(permission: String): Boolean

    suspend fun requestPermissions(permissions: Array<String>): Map<String, Boolean>

    suspend fun pickFile(mimeTypes: Array<String>): Uri?

    suspend fun pickMultipleFiles(mimeTypes: Array<String>, maxItems: Int): List<Uri>

    suspend fun pickImage(): Uri?

    suspend fun pickAudio(): Uri?

    suspend fun captureImage(targetUri: Uri): Boolean

    suspend fun captureVideo(targetUri: Uri): Boolean

    suspend fun openSettings(): Boolean

    fun createTempImageUri(): Uri

    fun createTempVideoUri(): Uri

    fun attachActivity(activity: ComponentActivity)

    fun detachActivity(activity: ComponentActivity)

    fun hasActivity(): Boolean
}

@Singleton
class ActivityResultBridgeImpl @Inject constructor(
    @ApplicationContext private val appContext: Context,
    private val logger: Logger
) : ActivityResultBridge {

    private val mutex = Mutex()

    @Volatile
    private var attachedActivity: ComponentActivity? = null

    private var permissionLauncher: ActivityResultLauncher<String>? = null
    private var multiPermissionLauncher: ActivityResultLauncher<Array<String>>? = null
    private var getContentLauncher: ActivityResultLauncher<String>? = null
    private var openDocumentLauncher: ActivityResultLauncher<Array<String>>? = null
    private var getMultipleContentsLauncher: ActivityResultLauncher<String>? = null
    private var captureImageLauncher: ActivityResultLauncher<Uri>? = null
    private var captureVideoLauncher: ActivityResultLauncher<Uri>? = null
    private var settingsLauncher: ActivityResultLauncher<Intent>? = null

    private var permissionCallback: ((Boolean) -> Unit)? = null
    private var multiPermissionCallback: ((Map<String, Boolean>) -> Unit)? = null
    private var getContentCallback: ((Uri?) -> Unit)? = null
    private var openDocumentCallback: ((Uri?) -> Unit)? = null
    private var multipleContentsCallback: ((List<Uri>) -> Unit)? = null
    private var captureImageCallback: ((Boolean) -> Unit)? = null
    private var captureVideoCallback: ((Boolean) -> Unit)? = null
    private var settingsCallback: ((Boolean) -> Unit)? = null

    override fun attachActivity(activity: ComponentActivity) {
        if (attachedActivity === activity) return
        detachActivity(attachedActivity ?: activity)
        attachedActivity = activity
        permissionLauncher = activity.registerForActivityResult(
            ActivityResultContracts.RequestPermission()
        ) { granted ->
            permissionCallback?.invoke(granted)
            permissionCallback = null
        }
        multiPermissionLauncher = activity.registerForActivityResult(
            ActivityResultContracts.RequestMultiplePermissions()
        ) { result ->
            multiPermissionCallback?.invoke(result)
            multiPermissionCallback = null
        }
        getContentLauncher = activity.registerForActivityResult(
            ActivityResultContracts.GetContent()
        ) { uri ->
            getContentCallback?.invoke(uri)
            getContentCallback = null
        }
        openDocumentLauncher = activity.registerForActivityResult(
            ActivityResultContracts.OpenDocument()
        ) { uri ->
            openDocumentCallback?.invoke(uri)
            openDocumentCallback = null
        }
        getMultipleContentsLauncher = activity.registerForActivityResult(
            ActivityResultContracts.GetMultipleContents()
        ) { uris ->
            multipleContentsCallback?.invoke(uris)
            multipleContentsCallback = null
        }
        captureImageLauncher = activity.registerForActivityResult(
            ActivityResultContracts.TakePicture()
        ) { ok ->
            captureImageCallback?.invoke(ok)
            captureImageCallback = null
        }
        captureVideoLauncher = activity.registerForActivityResult(
            ActivityResultContracts.TakeVideo()
        ) { thumbnail ->
            val success = thumbnail != null
            captureVideoCallback?.invoke(success)
            captureVideoCallback = null
        }
        settingsLauncher = activity.registerForActivityResult(
            ActivityResultContracts.StartActivityForResult()
        ) { result ->
            settingsCallback?.invoke(result.resultCode != 0)
            settingsCallback = null
        }
        logger.i(TAG, "ActivityResultBridge attached to ${activity.javaClass.simpleName}")
    }

    override fun detachActivity(activity: ComponentActivity) {
        if (attachedActivity !== activity) return
        runCatching { permissionLauncher?.unregister() }
        runCatching { multiPermissionLauncher?.unregister() }
        runCatching { getContentLauncher?.unregister() }
        runCatching { openDocumentLauncher?.unregister() }
        runCatching { getMultipleContentsLauncher?.unregister() }
        runCatching { captureImageLauncher?.unregister() }
        runCatching { captureVideoLauncher?.unregister() }
        runCatching { settingsLauncher?.unregister() }
        permissionLauncher = null
        multiPermissionLauncher = null
        getContentLauncher = null
        openDocumentLauncher = null
        getMultipleContentsLauncher = null
        captureImageLauncher = null
        captureVideoLauncher = null
        settingsLauncher = null
        attachedActivity = null
        logger.i(TAG, "ActivityResultBridge detached")
    }

    override fun hasActivity(): Boolean = attachedActivity != null

    override suspend fun requestPermission(permission: String): Boolean = mutex.withLock {
        val launcher = permissionLauncher ?: return@withLock false
        return@withLock suspendCancellableCoroutine { cont ->
            permissionCallback = { granted ->
                if (cont.isActive) cont.resume(granted)
            }
            cont.invokeOnCancellation { permissionCallback = null }
            launcher.launch(permission)
        }
    }

    override suspend fun requestPermissions(permissions: Array<String>): Map<String, Boolean> = mutex.withLock {
        val launcher = multiPermissionLauncher ?: return@withLock emptyMap()
        return@withLock suspendCancellableCoroutine { cont ->
            multiPermissionCallback = { result ->
                if (cont.isActive) cont.resume(result)
            }
            cont.invokeOnCancellation { multiPermissionCallback = null }
            launcher.launch(permissions)
        }
    }

    override suspend fun pickFile(mimeTypes: Array<String>): Uri? = mutex.withLock {
        if (mimeTypes.size == 1) {
            val launcher = getContentLauncher ?: return@withLock null
            return@withLock suspendCancellableCoroutine { cont ->
                getContentCallback = { uri ->
                    if (cont.isActive) cont.resume(uri)
                }
                cont.invokeOnCancellation { getContentCallback = null }
                launcher.launch(mimeTypes.first())
            }
        }
        val launcher = openDocumentLauncher ?: return@withLock null
        return@withLock suspendCancellableCoroutine { cont ->
            openDocumentCallback = { uri ->
                if (cont.isActive) cont.resume(uri)
            }
            cont.invokeOnCancellation { openDocumentCallback = null }
            launcher.launch(mimeTypes)
        }
    }

    override suspend fun pickMultipleFiles(mimeTypes: Array<String>, maxItems: Int): List<Uri> = mutex.withLock {
        val launcher = getMultipleContentsLauncher ?: return@withLock emptyList()
        return@withLock suspendCancellableCoroutine { cont ->
            multipleContentsCallback = { uris ->
                if (cont.isActive) cont.resume(uris)
            }
            cont.invokeOnCancellation { multipleContentsCallback = null }
            launcher.launch(mimeTypes.firstOrNull() ?: "*/*")
        }
    }

    override suspend fun pickImage(): Uri? = pickFile(arrayOf("image/*"))

    override suspend fun pickAudio(): Uri? = pickFile(arrayOf("audio/*"))

    override suspend fun captureImage(targetUri: Uri): Boolean = mutex.withLock {
        val launcher = captureImageLauncher ?: return@withLock false
        return@withLock suspendCancellableCoroutine { cont ->
            captureImageCallback = { ok ->
                if (cont.isActive) cont.resume(ok)
            }
            cont.invokeOnCancellation { captureImageCallback = null }
            launcher.launch(targetUri)
        }
    }

    override suspend fun captureVideo(targetUri: Uri): Boolean = mutex.withLock {
        val launcher = captureVideoLauncher ?: return@withLock false
        return@withLock suspendCancellableCoroutine { cont ->
            captureVideoCallback = { ok ->
                if (cont.isActive) cont.resume(ok)
            }
            cont.invokeOnCancellation { captureVideoCallback = null }
            launcher.launch(targetUri)
        }
    }

    override suspend fun openSettings(): Boolean = mutex.withLock {
        val launcher = settingsLauncher ?: return@withLock false
        return@withLock suspendCancellableCoroutine { cont ->
            settingsCallback = { ok ->
                if (cont.isActive) cont.resume(ok)
            }
            cont.invokeOnCancellation { settingsCallback = null }
            val intent = Intent(android.provider.Settings.ACTION_APPLICATION_DETAILS_SETTINGS).apply {
                data = Uri.fromParts("package", appContext.packageName, null)
            }
            launcher.launch(intent)
        }
    }

    override fun createTempImageUri(): Uri {
        val dir = File(appContext.cacheDir, "captured").apply { mkdirs() }
        val file = File(dir, "img_${System.currentTimeMillis()}.jpg")
        return FileProvider.getUriForFile(appContext, "${appContext.packageName}.fileprovider", file)
    }

    override fun createTempVideoUri(): Uri {
        val dir = File(appContext.cacheDir, "captured").apply { mkdirs() }
        val file = File(dir, "video_${System.currentTimeMillis()}.mp4")
        return FileProvider.getUriForFile(appContext, "${appContext.packageName}.fileprovider", file)
    }

    companion object {
        private const val TAG = "ActivityResultBridge"
    }
}
