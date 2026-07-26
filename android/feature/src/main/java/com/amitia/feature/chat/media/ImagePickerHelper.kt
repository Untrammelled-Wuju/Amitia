package com.amitia.feature.chat.media

import android.content.Context
import android.net.Uri
import androidx.activity.compose.ManagedActivityResultLauncher
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.PickVisualMediaRequest
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import java.io.File

class ImagePickerHelper(
    private val cacheDir: File,
    private val onPicked: (List<File>) -> Unit,
    private val onFailed: (Throwable) -> Unit
) {

    private val tempFiles = mutableListOf<File>()

    fun onPickVisualMediaResult(uris: List<Uri>, context: Context) {
        runCatching {
            val files = uris.mapNotNull { uri -> uriToFile(context, uri) }
            tempFiles.addAll(files)
            onPicked(files)
        }.onFailure(onFailed)
    }

    fun onTakePhotoResult(uri: Uri?, context: Context) {
        if (uri == null) return
        runCatching {
            val file = uriToFile(context, uri)
            if (file != null) {
                tempFiles.add(file)
                onPicked(listOf(file))
            }
        }.onFailure(onFailed)
    }

    fun newPhotoUri(): Uri {
        val file = File(cacheDir, "photo_${System.currentTimeMillis()}.jpg")
        return Uri.fromFile(file)
    }

    fun release() {
        tempFiles.clear()
    }

    private fun uriToFile(context: Context, uri: Uri): File? {
        return runCatching {
            val target = File(cacheDir, "img_${System.currentTimeMillis()}_${uri.lastPathSegment ?: "default"}.jpg")
            context.contentResolver.openInputStream(uri)?.use { input ->
                target.outputStream().use { output -> input.copyTo(output) }
            }
            target
        }.getOrNull()
    }
}

@Composable
fun rememberImagePickerLauncher(
    context: Context,
    cacheDir: File,
    onPicked: (List<File>) -> Unit,
    onFailed: (Throwable) -> Unit = {}
): ImagePickerHolder {
    val helper = remember(cacheDir) {
        ImagePickerHelper(cacheDir = cacheDir, onPicked = onPicked, onFailed = onFailed)
    }
    val launcher: ManagedActivityResultLauncher<PickVisualMediaRequest, List<Uri>> =
        rememberLauncherForActivityResult(
            contract = ActivityResultContracts.PickMultipleVisualMedia(maxItems = 9)
        ) { uris ->
            if (uris.isNotEmpty()) helper.onPickVisualMediaResult(uris, context)
        }
    val cameraLauncher: ManagedActivityResultLauncher<Uri, Boolean> =
        rememberLauncherForActivityResult(
            contract = ActivityResultContracts.TakePicture()
        ) { success ->
            if (success) helper.onTakePhotoResult(helper.newPhotoUri(), context)
        }
    return ImagePickerHolder(
        helper = helper,
        pickMedia = { launcher.launch(PickVisualMediaRequest(ActivityResultContracts.PickVisualMedia.ImageOnly)) },
        takePhoto = {
            val uri = helper.newPhotoUri()
            cameraLauncher.launch(uri)
        }
    )
}

class ImagePickerHolder(
    val helper: ImagePickerHelper,
    val pickMedia: () -> Unit,
    val takePhoto: () -> Unit
)
