package com.amitia.amitia_app

import android.Manifest
import android.app.Activity
import android.content.Intent
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.provider.Settings
import android.view.View
import android.view.ViewGroup
import com.amitia.amitia_app.nativeprovider.AndroidNativeBridgePlugin
import com.amitia.amitia_app.nativeprovider.AndroidNativeCompositionRoot
import com.amitia.amitia_app.runtime.bridge.RuntimeBridgePlugin
import com.amitia.amitia_app.realtime.RealtimeAudioPlugin
import com.amitia.amitia_app.realtime.RealtimeVisualPlugin
import com.amitia.amitia_app.update.AppUpdatePlugin
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.android.FlutterView
import io.flutter.embedding.engine.FlutterEngine
import androidx.core.app.ActivityCompat
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.lang.ref.WeakReference

class MainActivity : FlutterActivity() {
    companion object {
        @Volatile private var activeActivity: WeakReference<MainActivity>? = null
        private const val REQUEST_NOTIFICATION_POST = 4101
        private const val REQUEST_NOTIFICATION_SETTINGS = 4102
        private const val REQUEST_WORKSPACE_TREE = 4103

        fun currentActivity(): MainActivity? = activeActivity?.get()
    }

    private var imeInsetsSyncCallback: ImeInsetsSyncCallback? = null
    private var workspaceTreePending: CompletableDeferred<Pair<Uri, Int>?>? = null
    private var notificationPermissionPending: CompletableDeferred<Boolean>? = null
    private var notificationSettingsPending: CompletableDeferred<Unit>? = null

    suspend fun selectWorkspaceDocumentTree(): Pair<Uri, Int>? = withContext(Dispatchers.Main.immediate) {
        if (workspaceTreePending != null) {
            throw IllegalStateException("workspace directory picker is already open")
        }
        val pending = CompletableDeferred<Pair<Uri, Int>?>()
        workspaceTreePending = pending
        val intent = Intent(Intent.ACTION_OPEN_DOCUMENT_TREE).apply {
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
            addFlags(Intent.FLAG_GRANT_WRITE_URI_PERMISSION)
            addFlags(Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION)
            addFlags(Intent.FLAG_GRANT_PREFIX_URI_PERMISSION)
        }
        startActivityForResult(intent, REQUEST_WORKSPACE_TREE)
        pending.await()
    }

    suspend fun requestNotificationPostPermission(): Boolean = withContext(Dispatchers.Main.immediate) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU) return@withContext true
        if (checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) == android.content.pm.PackageManager.PERMISSION_GRANTED) {
            return@withContext true
        }
        if (notificationPermissionPending != null) {
            return@withContext notificationPermissionPending!!.await()
        }
        val pending = CompletableDeferred<Boolean>()
        notificationPermissionPending = pending
        ActivityCompat.requestPermissions(
            this@MainActivity,
            arrayOf(Manifest.permission.POST_NOTIFICATIONS),
            REQUEST_NOTIFICATION_POST,
        )
        pending.await()
    }

    suspend fun openAppNotificationSettings() = withContext(Dispatchers.Main.immediate) {
        val existing = notificationSettingsPending
        if (existing != null) {
            existing.await()
            return@withContext
        }
        val pending = CompletableDeferred<Unit>()
        notificationSettingsPending = pending
        val intent = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            Intent(Settings.ACTION_APP_NOTIFICATION_SETTINGS).apply {
                putExtra(Settings.EXTRA_APP_PACKAGE, packageName)
            }
        } else {
            Intent(
                Settings.ACTION_APPLICATION_DETAILS_SETTINGS,
                Uri.parse("package:$packageName"),
            )
        }
        startActivityForResult(intent, REQUEST_NOTIFICATION_SETTINGS)
        pending.await()
    }

    override fun onRequestPermissionsResult(
        requestCode: Int,
        permissions: Array<out String>,
        grantResults: IntArray,
    ) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults)
        if (requestCode != REQUEST_NOTIFICATION_POST) return
        val pending = notificationPermissionPending
        notificationPermissionPending = null
        pending?.complete(
            grantResults.isNotEmpty() &&
                grantResults[0] == android.content.pm.PackageManager.PERMISSION_GRANTED,
        )
    }

    @Deprecated("Deprecated in Java")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        when (requestCode) {
            REQUEST_NOTIFICATION_SETTINGS -> {
                val pending = notificationSettingsPending
                notificationSettingsPending = null
                pending?.complete(Unit)
            }
            REQUEST_WORKSPACE_TREE -> {
                val pending = workspaceTreePending
                workspaceTreePending = null
                if (pending == null || pending.isCompleted) return
                val uri = data?.data
                if (resultCode == Activity.RESULT_OK && uri != null) {
                    pending.complete(uri to (data?.flags ?: 0))
                } else {
                    pending.complete(null)
                }
            }
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        AndroidNativeCompositionRoot.initialize(applicationContext)
    }

    override fun onResume() {
        super.onResume()
        activeActivity = WeakReference(this)
    }

    override fun onPause() {
        if (activeActivity?.get() === this) activeActivity = null
        super.onPause()
    }

    override fun onDestroy() {
        if (activeActivity?.get() === this) activeActivity = null
        workspaceTreePending?.let { pending ->
            if (!pending.isCompleted) pending.complete(null)
        }
        workspaceTreePending = null
        notificationPermissionPending?.let { pending ->
            if (!pending.isCompleted) pending.complete(false)
        }
        notificationPermissionPending = null
        notificationSettingsPending?.let { pending ->
            if (!pending.isCompleted) pending.complete(Unit)
        }
        notificationSettingsPending = null
        super.onDestroy()
    }

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        flutterEngine.plugins.add(RuntimeBridgePlugin())
        flutterEngine.plugins.add(AndroidNativeBridgePlugin())
        flutterEngine.plugins.add(RealtimeAudioPlugin())
        flutterEngine.plugins.add(RealtimeVisualPlugin())
        flutterEngine.plugins.add(AppUpdatePlugin())
    }

    override fun onPostResume() {
        super.onPostResume()
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.R) return
        window.decorView.post {
            val flutterView = findFlutterView(window.decorView) ?: return@post
            imeInsetsSyncCallback?.remove()
            imeInsetsSyncCallback = ImeInsetsSyncCallback(flutterView).also { it.install() }
        }
    }

    private fun findFlutterView(view: View): FlutterView? {
        if (view is FlutterView) return view
        if (view !is ViewGroup) return null
        for (index in 0 until view.childCount) {
            findFlutterView(view.getChildAt(index))?.let { return it }
        }
        return null
    }
}
