package com.amitia.amitia_app.nativeprovider.accessibility

import android.content.Context
import android.content.Intent
import android.net.Uri
import android.provider.Settings
import androidx.core.content.FileProvider
import java.io.File

internal object AccessibilityProviderInstaller {

    const val PROVIDER_PACKAGE = "com.amitia.amitia_app.accessibility"
    private const val ASSET_PATH = "accessibility/amitia-accessibility.apk"

    fun installedVersion(context: Context): String? {
        return try {
            context.packageManager.getPackageInfo(PROVIDER_PACKAGE, 0).versionName
        } catch (_: Throwable) {
            null
        }
    }

    fun isInstalled(context: Context): Boolean = installedVersion(context) != null

    fun launchInstall(context: Context): Boolean {
        return try {
            val targetDir = File(context.cacheDir, "provider-install").apply { mkdirs() }
            val target = File(targetDir, "amitia-accessibility.apk")
            context.assets.open(ASSET_PATH).use { input ->
                target.outputStream().use { output -> input.copyTo(output) }
            }
            val uri = FileProvider.getUriForFile(
                context,
                "${context.packageName}.fileprovider",
                target,
            )
            val intent = Intent(Intent.ACTION_INSTALL_PACKAGE).apply {
                data = uri
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
                putExtra(Intent.EXTRA_NOT_UNKNOWN_SOURCE, true)
                putExtra(Intent.EXTRA_RETURN_RESULT, false)
            }
            context.startActivity(intent)
            true
        } catch (_: Throwable) {
            false
        }
    }

    fun openProviderSettings(context: Context): Boolean {
        val intent = if (isInstalled(context)) {
            Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS)
                .setData(Uri.parse("package:$PROVIDER_PACKAGE"))
        } else {
            Intent(Settings.ACTION_SECURITY_SETTINGS)
        }
        intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        return try {
            context.startActivity(intent)
            true
        } catch (_: Throwable) {
            false
        }
    }
}
