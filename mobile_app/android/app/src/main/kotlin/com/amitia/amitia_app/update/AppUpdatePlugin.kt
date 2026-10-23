package com.amitia.amitia_app.update

import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.PackageInstaller
import android.net.Uri
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.provider.Settings
import android.util.Base64
import io.flutter.embedding.engine.plugins.FlutterPlugin
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel
import java.io.FileNotFoundException
import java.io.File
import java.security.KeyFactory
import java.security.MessageDigest
import java.security.Signature
import java.security.spec.X509EncodedKeySpec
import java.util.concurrent.Executors
import java.util.concurrent.ThreadLocalRandom

class AppUpdatePlugin : FlutterPlugin, MethodChannel.MethodCallHandler {
    companion object {
        const val CHANNEL_NAME = "com.amitia.app_update/control"
        private const val PREFS_NAME = "app_update"
        private const val PENDING_RESULT_KEY = "pending_install_result"
        private const val PUBLIC_KEY_ASSET = "app-update/manifest-public-key.pem"

        fun storeInstallResult(
            context: Context,
            statusCode: Int,
            status: String,
            message: String,
            sessionId: Int,
        ) {
            context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
                .edit()
                .putInt("statusCode", statusCode)
                .putString("status", status)
                .putString("message", message)
                .putInt("sessionId", sessionId)
                .putLong("timestamp", System.currentTimeMillis())
                .putBoolean(PENDING_RESULT_KEY, true)
                .apply()
        }
    }

    private lateinit var appContext: Context
    private var channel: MethodChannel? = null
    private val mainHandler = Handler(Looper.getMainLooper())
    private val executor = Executors.newSingleThreadExecutor()

    override fun onAttachedToEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        appContext = binding.applicationContext
        channel = MethodChannel(binding.binaryMessenger, CHANNEL_NAME).also {
            it.setMethodCallHandler(this)
        }
    }

    override fun onDetachedFromEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        channel?.setMethodCallHandler(null)
        channel = null
        executor.shutdown()
    }

    override fun onMethodCall(call: MethodCall, result: MethodChannel.Result) {
        when (call.method) {
            "getInstalledInfo" -> result.success(installedInfo())
            "verifyManifest" -> verifyManifest(call, result)
            "download" -> download(call, result)
            "getDownloadStatus" -> getDownloadStatus(call, result)
            "install" -> install(call, result)
            "getPendingInstallResult" -> result.success(consumeInstallResult())
            "openInstallPermissionSettings" -> openInstallPermissionSettings(result)
            else -> result.notImplemented()
        }
    }

    private fun installedInfo(): Map<String, Any?> {
        val packageInfo = appContext.packageManager.getPackageInfo(appContext.packageName, 0)
        val versionCode = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            packageInfo.longVersionCode
        } else {
            @Suppress("DEPRECATION")
            packageInfo.versionCode.toLong()
        }
        return mapOf(
            "packageName" to appContext.packageName,
            "versionName" to packageInfo.versionName.orEmpty(),
            "versionCode" to versionCode,
            "canInstallPackages" to canRequestPackageInstalls(),
        )
    }

    private fun verifyManifest(call: MethodCall, result: MethodChannel.Result) {
        try {
            val manifest = call.argument<String>("manifest").orEmpty()
            val signatureBase64 = call.argument<String>("signatureBase64").orEmpty()
            if (manifest.isEmpty() || signatureBase64.isEmpty()) {
                result.success(false)
                return
            }
            val publicKey = loadPublicKey()
            val verifier = Signature.getInstance("SHA256withRSA")
            verifier.initVerify(publicKey)
            verifier.update(manifest.toByteArray(Charsets.UTF_8))
            val verified = verifier.verify(Base64.decode(signatureBase64, Base64.DEFAULT))
            result.success(verified)
        } catch (error: Throwable) {
            result.error("MANIFEST_SIGNATURE_INVALID", error.message ?: "manifest signature verification failed", null)
        }
    }

    private fun download(call: MethodCall, result: MethodChannel.Result) {
        try {
            val url = call.argument<String>("url").orEmpty()
            val fileNameRaw = call.argument<String>("fileName").orEmpty()
            val expectedSize = call.argument<Number>("expectedSize")?.toLong() ?: -1L
            val expectedSha256 = call.argument<String>("expectedSha256").orEmpty().lowercase()
            val fileName = fileNameRaw.replace(Regex("[^A-Za-z0-9._-]"), "_")
            if (url.isEmpty() || fileName.isEmpty() || expectedSize <= 0L ||
                !expectedSha256.matches(Regex("[a-f0-9]{64}"))) {
                result.error("DOWNLOAD_INVALID_REQUEST", "download url, file name, size and hash are required", null)
                return
            }
            val uri = Uri.parse(url)
            if (uri.scheme != "https") {
                result.error("DOWNLOAD_INSECURE_URL", "update downloads require https", null)
                return
            }
            val downloadId = System.currentTimeMillis() * 1000 +
                ThreadLocalRandom.current().nextLong(0, 1000)
            val directory = appContext.getExternalFilesDir(android.os.Environment.DIRECTORY_DOWNLOADS)
                ?: File(appContext.filesDir, "downloads")
            directory.mkdirs()
            AppUpdateDownloadStore.initialize(
                appContext,
                downloadId,
                url,
                fileName,
                expectedSize,
                expectedSha256,
                File(directory, fileName).absolutePath,
            )
            SegmentedApkDownloadService.start(
                appContext,
                downloadId,
                url,
                fileName,
                expectedSize,
                expectedSha256,
            )
            result.success(downloadId)
        } catch (error: Throwable) {
            result.error("DOWNLOAD_START_FAILED", error.message ?: "download could not be started", null)
        }
    }

    private fun getDownloadStatus(call: MethodCall, result: MethodChannel.Result) {
        try {
            val downloadId = call.argument<Number>("downloadId")?.toLong()
            if (downloadId == null) {
                result.error("DOWNLOAD_INVALID_REQUEST", "downloadId is required", null)
                return
            }
            result.success(AppUpdateDownloadStore.status(appContext, downloadId))
        } catch (error: Throwable) {
            result.error("DOWNLOAD_STATUS_FAILED", error.message ?: "download status is unavailable", null)
        }
    }

    private fun install(call: MethodCall, result: MethodChannel.Result) {
        val downloadId = call.argument<Number>("downloadId")?.toLong()
        val expectedSha256 = call.argument<String>("expectedSha256").orEmpty().lowercase()
        val expectedSize = call.argument<Number>("expectedSize")?.toLong() ?: -1L
        if (downloadId == null || expectedSha256.isEmpty() || expectedSize < 0L) {
            result.error("INSTALL_INVALID_REQUEST", "downloadId, expectedSha256 and expectedSize are required", null)
            return
        }
        if (!canRequestPackageInstalls()) {
            result.error("INSTALL_PERMISSION_REQUIRED", "install unknown apps permission is required", null)
            return
        }

        runIo(result) {
            val uri = downloadedUri(downloadId)
                ?: throw IllegalStateException("downloaded update file is unavailable")
            val packageInstaller = appContext.packageManager.packageInstaller
            val params = PackageInstaller.SessionParams(
                PackageInstaller.SessionParams.MODE_FULL_INSTALL,
            )
            val sessionId = packageInstaller.createSession(params)
            val session = packageInstaller.openSession(sessionId)
            try {
                val digest = MessageDigest.getInstance("SHA-256")
                val input = appContext.contentResolver.openInputStream(uri)
                    ?: throw FileNotFoundException("downloaded update file is unavailable")
                var total = 0L
                input.use { source ->
                    session.openWrite("base.apk", 0L, expectedSize).use { output ->
                        val buffer = ByteArray(1024 * 128)
                        while (true) {
                            val read = source.read(buffer)
                            if (read <= 0) break
                            digest.update(buffer, 0, read)
                            output.write(buffer, 0, read)
                            total += read
                        }
                        output.flush()
                    }
                }
                val actualSha256 = digest.digest().joinToString("") { "%02x".format(it) }
                if (total != expectedSize) {
                    session.abandon()
                    throw IllegalStateException("downloaded update size mismatch")
                }
                if (actualSha256 != expectedSha256) {
                    session.abandon()
                    throw IllegalStateException("downloaded update hash mismatch")
                }
                val callbackIntent = Intent(appContext, AppUpdateInstallReceiver::class.java)
                    .setAction("$CHANNEL_NAME.INSTALL_RESULT")
                val pendingIntent = PendingIntent.getBroadcast(
                    appContext,
                    sessionId,
                    callbackIntent,
                    PendingIntent.FLAG_UPDATE_CURRENT or pendingIntentMutabilityFlag(),
                )
                session.commit(pendingIntent.intentSender)
                AppUpdateDownloadStore.localFile(appContext, downloadId)?.delete()
            } catch (error: Throwable) {
                runCatching { session.abandon() }
                throw error
            } finally {
                session.close()
            }
            mapOf("started" to true, "sessionId" to sessionId)
        }
    }

    private fun downloadedUri(downloadId: Long): Uri? {
        val status = AppUpdateDownloadStore.status(appContext, downloadId)
        if ((status["statusCode"] as? Int) != 8) {
            throw IllegalStateException("update download is not complete")
        }
        val file = AppUpdateDownloadStore.localFile(appContext, downloadId)
            ?: throw IllegalStateException("downloaded update file is unavailable")
        if (!file.isFile) throw FileNotFoundException("downloaded update file is unavailable")
        return Uri.fromFile(file)
    }

    private fun openInstallPermissionSettings(result: MethodChannel.Result) {
        try {
            val intent = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                Intent(
                    Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES,
                    Uri.parse("package:${appContext.packageName}"),
                )
            } else {
                Intent(Settings.ACTION_SECURITY_SETTINGS)
            }
            intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            appContext.startActivity(intent)
            result.success(true)
        } catch (error: Throwable) {
            result.error("INSTALL_SETTINGS_FAILED", error.message ?: "install settings could not be opened", null)
        }
    }

    private fun consumeInstallResult(): Map<String, Any?>? {
        val prefs = appContext.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
        if (!prefs.getBoolean(PENDING_RESULT_KEY, false)) return null
        val result = mapOf(
            "statusCode" to prefs.getInt("statusCode", PackageInstaller.STATUS_FAILURE),
            "status" to prefs.getString("status", "failed"),
            "message" to prefs.getString("message", ""),
            "sessionId" to prefs.getInt("sessionId", -1),
            "timestamp" to prefs.getLong("timestamp", 0L),
        )
        prefs.edit().remove(PENDING_RESULT_KEY).apply()
        return result
    }

    private fun canRequestPackageInstalls(): Boolean {
        return Build.VERSION.SDK_INT < Build.VERSION_CODES.O ||
            appContext.packageManager.canRequestPackageInstalls()
    }

    private fun loadPublicKey() = appContext.assets.open(PUBLIC_KEY_ASSET).use { input ->
        val pem = input.readBytes().toString(Charsets.UTF_8)
        val encoded = pem
            .replace("-----BEGIN PUBLIC KEY-----", "")
            .replace("-----END PUBLIC KEY-----", "")
            .replace(Regex("\\s"), "")
        KeyFactory.getInstance("RSA").generatePublic(
            X509EncodedKeySpec(Base64.decode(encoded, Base64.DEFAULT)),
        )
    }

    private fun runIo(result: MethodChannel.Result, block: () -> Any?) {
        executor.execute {
            try {
                val value = block()
                mainHandler.post { result.success(value) }
            } catch (error: Throwable) {
                mainHandler.post {
                    result.error("APP_UPDATE_ERROR", error.message ?: "app update failed", null)
                }
            }
        }
    }

    private fun pendingIntentMutabilityFlag(): Int {
        return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            PendingIntent.FLAG_MUTABLE
        } else {
            0
        }
    }

}
