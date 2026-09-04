package com.amitia.amitia_app.nativeprovider.desktoppet

import android.content.Context
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Color
import android.graphics.PixelFormat
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.provider.Settings
import android.util.LruCache
import android.view.Gravity
import android.view.MotionEvent
import android.view.View
import android.view.ViewConfiguration
import android.view.WindowManager
import android.widget.ImageView
import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import java.security.MessageDigest
import java.util.ArrayDeque
import java.util.UUID
import kotlin.math.roundToInt

/**
 * Android Runtime V2 renderer for one device-scoped desktop pet.
 *
 * State/command authority stays in the Go Runtime V2 service. This class owns
 * only the Android WindowManager surface, Package V2 frame decoding/playback,
 * and direct manipulation facts that Runtime V2 reports back as actual state.
 * Package paths are resolved beneath filesDir/amitia/data and traversal fails
 * closed.
 */
internal class DesktopPetRendererNativeHandler(
    context: Context,
) : AndroidNativeOperationHandler {

    private data class FrameSpec(
        val file: File,
        val durationMs: Long,
    )

    private data class IntegrityFileEntry(
        val path: String,
        val sha256: String,
        val bytes: Long,
    )

    private data class ReturnRule(
        val type: String,
        val actionKey: String,
    )

    private data class ActionSpec(
        val key: String,
        val frames: List<FrameSpec>,
        val playbackMode: String,
        val returnTo: ReturnRule,
        val interruptible: Boolean,
        val interruptAfterMs: Long,
        val minimumPlayMs: Long,
        val maximumPlayMs: Long?,
    ) {
        val singleCycleDurationMs: Long
            get() = frames.sumOf { it.durationMs.coerceAtLeast(MIN_FRAME_DURATION_MS) }
    }

    private data class LoadedPet(
        val installationId: String,
        val installRoot: File,
        val characterId: String,
        val petId: String,
        val releaseId: String,
        val releaseVersion: String,
        val manifestHash: String,
        val contentRootHash: String,
        val defaultActionKey: String,
        val actions: Map<String, ActionSpec>,
        val canvasWidth: Int,
        val canvasHeight: Int,
    )

    private val appContext = context.applicationContext
    private val dataRoot = File(File(appContext.filesDir, "amitia"), "data")
    private val windowManager = appContext.getSystemService(Context.WINDOW_SERVICE) as WindowManager
    private val mainHandler = Handler(Looper.getMainLooper())

    private var imageView: ImageView? = null
    private var layoutParams: WindowManager.LayoutParams? = null
    private var loadedPet: LoadedPet? = null
    private var visible = false
    private var paused = false
    private var alpha = 1f
    private var currentActionKey = ""
    private var previousActionKey = ""
    private var playbackId = ""
    private var playbackStartedAt = 0L
    private var pauseStartedAt = 0L
    private var pausedAccumulatedMs = 0L
    private var frameIndex = 0
    private var cycleIndex = 0
    private var animationRunnable: Runnable? = null
    private var playbackRate = 1.0
    private var currentPlaybackMode = ""
    private var currentInterruptible = true
    private var playbackActive = false
    private var lastCompletedPlaybackId = ""
    private var lastCompletionReason = ""
    private var lastCompletedPlayedMs = 0L
    private var lastCompletedCycleIndex = 0
    private var positionRevision = 0L
    private val interactionEvents = ArrayDeque<Map<String, Any?>>()
    private val bitmapCache = object : LruCache<String, Bitmap>(BITMAP_CACHE_KB) {
        override fun sizeOf(key: String, value: Bitmap): Int =
            (value.allocationByteCount / 1024).coerceAtLeast(1)
    }

    override val operations: Set<String> = setOf(
        OP_STATUS,
        OP_LOAD,
        OP_UNLOAD,
        OP_SHOW,
        OP_HIDE,
        OP_SETTINGS,
        OP_PLAY,
        OP_STOP,
        OP_PAUSE,
        OP_RESUME,
        OP_RECENTER,
        OP_DRAIN_EVENTS,
    )

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse {
        if (request.operation == OP_LOAD) {
            return load(request)
        }
        return withContext(Dispatchers.Main.immediate) {
            when (request.operation) {
                OP_STATUS -> status(request)
                OP_UNLOAD -> unload(request)
                OP_SHOW -> show(request)
                OP_HIDE -> hide(request)
                OP_SETTINGS -> settings(request)
                OP_PLAY -> play(request)
                OP_STOP -> stop(request)
                OP_PAUSE -> pause(request)
                OP_RESUME -> resume(request)
                OP_RECENTER -> recenter(request)
                OP_DRAIN_EVENTS -> drainEvents(request)
                else -> failure(
                    request,
                    NativeBridgeProtocol.ERR_OPERATION_NOT_SUPPORTED,
                    "unsupported desktop pet renderer operation: ${request.operation}",
                )
            }
        }
    }

    private fun status(request: NativeBridgeRequest): NativeBridgeResponse =
        success(request, stateMap() + mapOf("supported" to true, "permissionGranted" to overlayPermissionGranted()))

    private suspend fun load(request: NativeBridgeRequest): NativeBridgeResponse {
        if (!overlayPermissionGranted()) {
            return failure(
                request,
                "OVERLAY_PERMISSION_REQUIRED",
                "overlay permission not granted",
                "OVERLAY_PERMISSION_REQUIRED",
            )
        }
        val installationId = request.payload.string("installationId")
        val installPath = request.payload.string("installPath")
        val authoritativeCharacterId = request.payload.string("characterId")
        if (installationId.isBlank() || installPath.isBlank()) {
            return failure(
                request,
                "DESKTOP_PET_RENDERER_INVALID_REQUEST",
                "installationId and installPath are required",
            )
        }

        val pet = try {
            withContext(Dispatchers.IO) {
                loadPackage(
                    installationId = installationId,
                    installPath = installPath,
                    authoritativeCharacterId = authoritativeCharacterId,
                    expectedPetId = request.payload.string("petId"),
                    expectedReleaseId = request.payload.string("releaseId"),
                    expectedReleaseVersion = request.payload.string("releaseVersion"),
                    expectedManifestHash = request.payload.string("manifestHash"),
                    expectedContentRootHash = request.payload.string("contentRootHash"),
                    authoritativeManifestJson = request.payload.string("authoritativeManifestJson"),
                    runtimeVersion = request.payload.string("runtimeVersion").ifBlank { RUNTIME_VERSION },
                )
            }
        } catch (error: Exception) {
            return failure(
                request,
                "DESKTOP_PET_PACKAGE_LOAD_FAILED",
                error.message ?: "failed to load desktop pet package",
            )
        }

        return withContext(Dispatchers.Main.immediate) {
            val requestedWidth = request.payload.intOrNull("width")
            val requestedHeight = request.payload.intOrNull("height")
            if (requestedWidth != null && requestedWidth !in MIN_PET_DP..MAX_PET_DP) {
                return@withContext failure(
                    request,
                    "DESKTOP_PET_SIZE_UNSUPPORTED",
                    "desktop pet width must be within ${MIN_PET_DP}..${MAX_PET_DP}dp; requested=$requestedWidth",
                )
            }
            if (requestedHeight != null && requestedHeight !in MIN_PET_DP..MAX_PET_DP) {
                return@withContext failure(
                    request,
                    "DESKTOP_PET_SIZE_UNSUPPORTED",
                    "desktop pet height must be within ${MIN_PET_DP}..${MAX_PET_DP}dp; requested=$requestedHeight",
                )
            }
            stopAnimation()
            removeSurface()
            bitmapCache.evictAll()
            interactionEvents.clear()
            loadedPet = pet
            resetPlaybackState(clearTerminal = true)
            paused = false
            alpha = request.payload.float("alpha", 1f).coerceIn(0.2f, 1f)

            val width = requestedWidth ?: pet.canvasWidth.coerceIn(MIN_PET_DP, MAX_PET_DP)
            val height = requestedHeight ?: pet.canvasHeight.coerceIn(MIN_PET_DP, MAX_PET_DP)
            val x = request.payload.int("x", DEFAULT_X_DP)
            val y = request.payload.int("y", DEFAULT_Y_DP)

            val view = ImageView(appContext).apply {
                setBackgroundColor(Color.TRANSPARENT)
                scaleType = ImageView.ScaleType.FIT_CENTER
                adjustViewBounds = false
                this.alpha = this@DesktopPetRendererNativeHandler.alpha
                visibility = View.GONE
            }
            val params = buildLayoutParams(width, height, x, y)
            installDragHandler(view, params)

            try {
                windowManager.addView(view, params)
                imageView = view
                layoutParams = params
                visible = false
                val initialAction = request.payload.string("actionKey").ifBlank { pet.defaultActionKey }
                val playResult = startAction(
                    actionKey = initialAction,
                    requestedPlaybackRate = 1.0,
                    forceLoop = isLoopingDefaultAction(pet, initialAction),
                    clearTerminal = true,
                )
                success(
                    request,
                    stateMap() + mapOf(
                        "loaded" to true,
                        "initialAction" to initialAction,
                        "playbackMode" to playResult["playbackMode"],
                    ),
                )
            } catch (error: Exception) {
                stopAnimation()
                removeSurface()
                bitmapCache.evictAll()
                loadedPet = null
                resetPlaybackState(clearTerminal = true)
                failure(
                    request,
                    "DESKTOP_PET_OVERLAY_CREATE_FAILED",
                    "failed to create desktop pet overlay: ${error.message}",
                )
            }
        }
    }

    private fun unload(request: NativeBridgeRequest): NativeBridgeResponse {
        stopAnimation()
        removeSurface()
        bitmapCache.evictAll()
        interactionEvents.clear()
        loadedPet = null
        resetPlaybackState(clearTerminal = true)
        visible = false
        paused = false
        return success(request, mapOf("unloaded" to true))
    }

    private fun show(request: NativeBridgeRequest): NativeBridgeResponse {
        if (!overlayPermissionGranted()) {
            return failure(
                request,
                "OVERLAY_PERMISSION_REQUIRED",
                "overlay permission not granted",
                "OVERLAY_PERMISSION_REQUIRED",
            )
        }
        val view = imageView
            ?: return failure(request, "DESKTOP_PET_RENDERER_NOT_LOADED", "desktop pet renderer is not loaded")
        view.visibility = View.VISIBLE
        visible = true
        return success(request, stateMap())
    }

    private fun hide(request: NativeBridgeRequest): NativeBridgeResponse {
        val view = imageView
            ?: return failure(request, "DESKTOP_PET_RENDERER_NOT_LOADED", "desktop pet renderer is not loaded")
        view.visibility = View.GONE
        visible = false
        return success(request, stateMap())
    }

    private fun settings(request: NativeBridgeRequest): NativeBridgeResponse {
        val view = imageView
            ?: return failure(request, "DESKTOP_PET_RENDERER_NOT_LOADED", "desktop pet renderer is not loaded")
        val params = layoutParams
            ?: return failure(request, "DESKTOP_PET_RENDERER_NOT_LOADED", "desktop pet renderer layout is unavailable")
        val requestedWidth = request.payload.intOrNull("width")
        val requestedHeight = request.payload.intOrNull("height")
        if (requestedWidth != null && requestedWidth !in MIN_PET_DP..MAX_PET_DP) {
            return failure(
                request,
                "DESKTOP_PET_SIZE_UNSUPPORTED",
                "desktop pet width must be within ${MIN_PET_DP}..${MAX_PET_DP}dp; requested=$requestedWidth",
            )
        }
        if (requestedHeight != null && requestedHeight !in MIN_PET_DP..MAX_PET_DP) {
            return failure(
                request,
                "DESKTOP_PET_SIZE_UNSUPPORTED",
                "desktop pet height must be within ${MIN_PET_DP}..${MAX_PET_DP}dp; requested=$requestedHeight",
            )
        }
        var positionChanged = false
        request.payload.floatOrNull("alpha")?.let {
            alpha = it.coerceIn(0.2f, 1f)
            view.alpha = alpha
        }
        requestedWidth?.let { params.width = dp(it) }
        requestedHeight?.let { params.height = dp(it) }
        request.payload.intOrNull("x")?.let {
            params.gravity = Gravity.TOP or Gravity.START
            params.x = dp(it)
            positionChanged = true
        }
        request.payload.intOrNull("y")?.let {
            params.gravity = Gravity.TOP or Gravity.START
            params.y = dp(it)
            positionChanged = true
        }
        if (positionChanged) positionRevision++
        return try {
            windowManager.updateViewLayout(view, params)
            success(request, stateMap())
        } catch (error: Exception) {
            failure(
                request,
                "DESKTOP_PET_SETTINGS_APPLY_FAILED",
                "failed to apply renderer settings: ${error.message}",
            )
        }
    }

    private fun play(request: NativeBridgeRequest): NativeBridgeResponse {
        val actionKey = request.payload.string("actionKey")
        if (actionKey.isBlank()) {
            return failure(request, "MISSING_ACTION_KEY", "actionKey is required")
        }
        val pet = loadedPet
            ?: return failure(request, "PET_NOT_READY", "desktop pet renderer is not ready")
        if (imageView == null) {
            return failure(request, "PET_NOT_READY", "desktop pet renderer surface is not ready")
        }
        val action = pet.actions[actionKey]
            ?: return failure(request, "ACTION_NOT_FOUND", "action $actionKey is unavailable")
        val requestedRate = request.payload.double("playbackRate", 1.0).coerceIn(0.25, 4.0)
        val requestedInterruptible = request.payload.booleanOrNull("interruptible")
        val result = try {
            startAction(
                actionKey = actionKey,
                requestedPlaybackRate = requestedRate,
                forceLoop = null,
                clearTerminal = true,
                requestedInterruptible = requestedInterruptible,
            )
        } catch (error: Exception) {
            return failure(request, "ACTION_NOT_FOUND", error.message ?: "action is unavailable")
        }
        return success(request, stateMap() + result)
    }

    private fun stop(request: NativeBridgeRequest): NativeBridgeResponse {
        val pet = loadedPet
            ?: return failure(request, "DESKTOP_PET_RENDERER_NOT_LOADED", "desktop pet renderer is not loaded")
        val previousPlaybackId = playbackId
        val previousPlayedMs = playedMs()
        stopAnimation()
        if (previousPlaybackId.isNotEmpty()) {
            lastCompletedPlaybackId = previousPlaybackId
            lastCompletionReason = "stopped"
            lastCompletedPlayedMs = previousPlayedMs
            lastCompletedCycleIndex = cycleIndex.coerceAtLeast(1)
        }
        playbackActive = false
        startAction(
            actionKey = pet.defaultActionKey,
            requestedPlaybackRate = 1.0,
            forceLoop = isLoopingDefaultAction(pet, pet.defaultActionKey),
            clearTerminal = false,
        )
        return success(
            request,
            stateMap() + mapOf(
                "stoppedPlaybackId" to previousPlaybackId,
                "stoppedPlayedMs" to previousPlayedMs,
            ),
        )
    }

    private fun pause(request: NativeBridgeRequest): NativeBridgeResponse {
        if (loadedPet == null) {
            return failure(request, "DESKTOP_PET_RENDERER_NOT_LOADED", "desktop pet renderer is not loaded")
        }
        if (!paused) {
            paused = true
            pauseStartedAt = System.currentTimeMillis()
        }
        return success(request, stateMap())
    }

    private fun resume(request: NativeBridgeRequest): NativeBridgeResponse {
        if (loadedPet == null) {
            return failure(request, "DESKTOP_PET_RENDERER_NOT_LOADED", "desktop pet renderer is not loaded")
        }
        if (paused) {
            val now = System.currentTimeMillis()
            if (pauseStartedAt > 0L) {
                pausedAccumulatedMs += (now - pauseStartedAt).coerceAtLeast(0L)
            }
            pauseStartedAt = 0L
            paused = false
        }
        return success(request, stateMap())
    }

    private fun recenter(request: NativeBridgeRequest): NativeBridgeResponse {
        val view = imageView
            ?: return failure(request, "DESKTOP_PET_RENDERER_NOT_LOADED", "desktop pet renderer is not loaded")
        val params = layoutParams
            ?: return failure(request, "DESKTOP_PET_RENDERER_NOT_LOADED", "desktop pet renderer layout is unavailable")
        val metrics = appContext.resources.displayMetrics
        params.gravity = Gravity.TOP or Gravity.START
        params.x = ((metrics.widthPixels - params.width) / 2).coerceAtLeast(0)
        params.y = ((metrics.heightPixels - params.height) / 2).coerceAtLeast(0)
        positionRevision++
        return try {
            windowManager.updateViewLayout(view, params)
            success(request, stateMap())
        } catch (error: Exception) {
            failure(request, "DESKTOP_PET_RECENTER_FAILED", "failed to recenter desktop pet: ${error.message}")
        }
    }

    private fun drainEvents(request: NativeBridgeRequest): NativeBridgeResponse {
        val drained = ArrayList<Map<String, Any?>>(interactionEvents.size)
        while (interactionEvents.isNotEmpty()) {
            drained.add(interactionEvents.removeFirst())
        }
        return success(request, mapOf("events" to drained))
    }

    private fun loadPackage(
        installationId: String,
        installPath: String,
        authoritativeCharacterId: String,
        expectedPetId: String,
        expectedReleaseId: String,
        expectedReleaseVersion: String,
        expectedManifestHash: String,
        expectedContentRootHash: String,
        authoritativeManifestJson: String,
        runtimeVersion: String,
    ): LoadedPet {
        val installRoot = resolveDataPath(installPath)
        require(installRoot.isDirectory) { "installation root does not exist: $installPath" }
        val manifestFile = File(installRoot, "manifest.json")
        require(manifestFile.isFile) { "manifest.json not found for installation $installationId" }
        val manifest = JSONObject(manifestFile.readText())
        require(authoritativeManifestJson.isNotBlank()) {
            "authoritative release manifest is missing"
        }
        val authoritativeManifest = JSONObject(authoritativeManifestJson)
        require(jsonStructurallyEqual(manifest, authoritativeManifest)) {
            "installed manifest does not match release authority"
        }
        val schema = manifest.optInt("schemaVersion", 0)
        require(schema == PACKAGE_SCHEMA_VERSION) {
            "unsupported desktop pet manifest schemaVersion=$schema; Package V2 is required"
        }
        require(manifest.optString("manifestFormat") == PACKAGE_MANIFEST_FORMAT) {
            "invalid desktop pet manifestFormat"
        }

        val petId = manifest.optString("petId").trim()
        val releaseId = manifest.optString("releaseId").trim()
        val releaseVersion = manifest.optString("version").trim()
        require(petId.isNotEmpty() && releaseId.isNotEmpty() && releaseVersion.isNotEmpty()) {
            "manifest petId/releaseId/version is missing"
        }
        require(expectedPetId.isBlank() || expectedPetId == petId) { "manifest petId does not match installation authority" }
        require(expectedReleaseId.isBlank() || expectedReleaseId == releaseId) { "manifest releaseId does not match installation authority" }
        require(expectedReleaseVersion.isBlank() || expectedReleaseVersion == releaseVersion) {
            "manifest version does not match release authority"
        }

        val compatibility = manifest.optJSONObject("compatibility")
            ?: error("manifest compatibility is missing")
        require(compatibility.optString("renderMode").trim() == "sprite") {
            "Android desktop pet renderer only supports sprite renderMode"
        }
        val minRuntimeVersion = compatibility.optString("minRuntimeVersion").trim()
        val maxRuntimeVersion = compatibility.optString("maxRuntimeVersion").trim()
        require(isSemVer(runtimeVersion) && isSemVer(minRuntimeVersion)) {
            "runtime compatibility version is invalid"
        }
        require(compareSemVer(runtimeVersion, minRuntimeVersion) >= 0) {
            "desktop pet requires runtime >= $minRuntimeVersion"
        }
        if (maxRuntimeVersion.isNotEmpty()) {
            require(isSemVer(maxRuntimeVersion) && compareSemVer(runtimeVersion, maxRuntimeVersion) <= 0) {
                "desktop pet requires runtime <= $maxRuntimeVersion"
            }
        }

        val integrity = manifest.optJSONObject("integrity") ?: error("manifest integrity is missing")
        require(integrity.optString("algorithm").trim() == INTEGRITY_ALGORITHM) {
            "unsupported package integrity algorithm"
        }
        val manifestHash = integrity.optString("manifestHash").trim()
        val contentRootHash = integrity.optString("contentRootHash").trim()
        require(isLowerHexSha256(manifestHash) && isLowerHexSha256(contentRootHash)) {
            "manifest integrity hashes are invalid"
        }
        require(expectedManifestHash.isBlank() || hashesEqual(expectedManifestHash, manifestHash)) {
            "manifest hash does not match release authority"
        }
        require(expectedContentRootHash.isBlank() || hashesEqual(expectedContentRootHash, contentRootHash)) {
            "content root hash does not match release authority"
        }
        val integrityEntries = verifyIntegrityFiles(installRoot, integrity)
        val canonicalManifestData = canonicalManifestData(manifest)
        val recomputedManifestHash = sha256Hex(canonicalManifestData)
        require(hashesEqual(manifestHash, recomputedManifestHash)) {
            "manifest canonical hash mismatch"
        }
        val recomputedContentRootHash = computeContentRootHash(
            integrityEntries,
            recomputedManifestHash,
            canonicalManifestData.size.toLong(),
        )
        require(hashesEqual(contentRootHash, recomputedContentRootHash)) {
            "content root tree hash mismatch"
        }

        val binding = manifest.optJSONObject("binding") ?: error("manifest binding is missing")
        val bindingPolicy = binding.optString("policy").trim()
        require(bindingPolicy in SUPPORTED_BINDING_POLICIES) { "unsupported binding policy: $bindingPolicy" }
        val bindingCharacterId = binding.optString("sourceCharacterId").trim()
        val characterId = authoritativeCharacterId.trim()
        require(characterId.isNotEmpty()) { "installation character identity is missing" }
        if (bindingPolicy == "bound" || bindingPolicy == "legacy_inferred") {
            require(bindingCharacterId.isNotEmpty() && bindingCharacterId == characterId) {
                "package binding character does not match installation authority"
            }
        }

        val canvas = manifest.optJSONObject("canvas") ?: error("manifest canvas is missing")
        require(canvas.optString("coordinateSystem").trim() == "top-left") {
            "unsupported canvas coordinateSystem"
        }
        val canvasWidth = canvas.optInt("width", 0)
        val canvasHeight = canvas.optInt("height", 0)
        require(canvasWidth in 1..MAX_CANVAS_PX && canvasHeight in 1..MAX_CANVAS_PX) {
            "manifest canvas dimensions are invalid"
        }

        val actionsJson = manifest.optJSONArray("actions") ?: error("manifest actions are missing")
        require(actionsJson.length() > 0) { "manifest actions are empty" }
        val actions = linkedMapOf<String, ActionSpec>()
        for (index in 0 until actionsJson.length()) {
            val item = actionsJson.optJSONObject(index)
                ?: error("manifest action $index is invalid")
            val key = item.optString("key").trim()
            val config = item.optString("config").trim()
            require(key.isNotEmpty() && config.isNotEmpty()) { "manifest action $index is incomplete" }
            require(!actions.containsKey(key)) { "duplicate manifest action key: $key" }
            val action = loadAction(installRoot, key, config)
            val manifestMode = item.optString("playbackMode").trim().lowercase()
            require(manifestMode.isNotEmpty() && manifestMode == action.playbackMode) {
                "action playbackMode mismatch for $key"
            }
            val frameCount = item.optInt("frameCount", 0)
            require(frameCount == action.frames.size) { "action frameCount mismatch for $key" }
            actions[key] = action
        }
        val defaultAction = manifest.optString("defaultAction").trim()
        require(defaultAction.isNotEmpty() && actions.containsKey(defaultAction)) {
            "default action is missing or unavailable: $defaultAction"
        }

        return LoadedPet(
            installationId = installationId,
            installRoot = installRoot,
            characterId = characterId,
            petId = petId,
            releaseId = releaseId,
            releaseVersion = releaseVersion,
            manifestHash = manifestHash,
            contentRootHash = contentRootHash,
            defaultActionKey = defaultAction,
            actions = actions,
            canvasWidth = canvasWidth,
            canvasHeight = canvasHeight,
        )
    }

    private fun verifyIntegrityFiles(installRoot: File, integrity: JSONObject): List<IntegrityFileEntry> {
        val files = integrity.optJSONArray("files") ?: error("integrity.files is missing")
        val expectedFileCount = integrity.optInt("fileCount", -1)
        val expectedTotalBytes = integrity.optLong("totalBytes", -1L)
        require(expectedFileCount == files.length() && expectedFileCount > 0) {
            "integrity fileCount does not match files"
        }
        require(expectedTotalBytes >= 0L) { "integrity totalBytes is invalid" }

        val seen = hashSetOf<String>()
        val verifiedEntries = ArrayList<IntegrityFileEntry>(files.length())
        var actualTotalBytes = 0L
        for (index in 0 until files.length()) {
            val entry = files.optJSONObject(index) ?: error("integrity file $index is invalid")
            val relativePath = entry.optString("path").trim()
            val expectedHash = entry.optString("sha256").trim()
            val expectedBytes = entry.optLong("bytes", -1L)
            require(relativePath.isNotEmpty() && isLowerHexSha256(expectedHash) && expectedBytes >= 0L) {
                "integrity file $index metadata is invalid"
            }
            require(seen.add(relativePath.lowercase())) { "duplicate integrity path: $relativePath" }
            val file = resolveUnder(installRoot, relativePath)
            require(file.isFile) { "integrity file is missing: $relativePath" }
            require(file.length() == expectedBytes) { "integrity file size mismatch: $relativePath" }
            require(hashesEqual(expectedHash, sha256Hex(file))) { "integrity file hash mismatch: $relativePath" }
            verifiedEntries.add(IntegrityFileEntry(relativePath, expectedHash, expectedBytes))
            actualTotalBytes += expectedBytes
        }
        require(actualTotalBytes == expectedTotalBytes) { "integrity totalBytes mismatch" }

        // Match the desktop PackageIntegrityVerifier: the installed package is
        // an exact declared tree. Extra files are rejected even when they are
        // not referenced by an action, so Android cannot become a looser asset
        // authority than Electron. manifest.json is the one pseudo-entry that
        // is intentionally not listed in integrity.files.
        val actualFiles = installRoot.walkTopDown()
            .filter { it.isFile }
            .map { it.relativeTo(installRoot).invariantSeparatorsPath }
            .filter { !it.equals("manifest.json", ignoreCase = true) }
            .map { it.lowercase() }
            .toSet()
        require(actualFiles == seen) {
            val undeclared = (actualFiles - seen).sorted()
            val missing = (seen - actualFiles).sorted()
            "package file declaration mismatch: undeclared=$undeclared missing=$missing"
        }
        return verifiedEntries
    }

    private fun canonicalManifestData(manifest: JSONObject): ByteArray {
        val clone = JSONObject(manifest.toString())
        val integrity = clone.optJSONObject("integrity")
            ?: error("manifest integrity is missing")
        integrity.put("manifestHash", "")
        integrity.put("contentRootHash", "")
        return canonicalJson(clone).toByteArray(Charsets.UTF_8)
    }

    /**
     * Exact Kotlin counterpart of packageformat.CanonicalJSON. Go canonicalizes
     * objects into a sorted array of {"k": key, "v": value} entries while
     * preserving array order. Manifest numeric fields are integral by schema;
     * non-finite numbers fail closed.
     */
    private fun canonicalJson(value: Any?): String = when (value) {
        null, JSONObject.NULL -> "null"
        is JSONObject -> {
            val keys = value.keys().asSequence().toList().sortedWith { left, right ->
                compareUtf8Strings(left, right)
            }
            keys.joinToString(prefix = "[", postfix = "]", separator = ",") { key ->
                "{\"k\":${goJsonQuote(key)},\"v\":${canonicalJson(value.opt(key))}}"
            }
        }
        is JSONArray -> (0 until value.length()).joinToString(
            prefix = "[",
            postfix = "]",
            separator = ",",
        ) { index -> canonicalJson(value.opt(index)) }
        is String -> goJsonQuote(value)
        is Boolean -> if (value) "true" else "false"
        is Byte, is Short, is Int, is Long -> value.toString()
        is Float -> canonicalFiniteNumber(value.toDouble())
        is Double -> canonicalFiniteNumber(value)
        is Number -> value.toString()
        else -> error("unsupported canonical JSON value: ${value::class.java.name}")
    }

    private fun canonicalFiniteNumber(value: Double): String {
        require(value.isFinite()) { "non-finite manifest number is invalid" }
        val asLong = value.toLong()
        if (value == asLong.toDouble()) return asLong.toString()
        // Package V2 currently contains integer numeric fields. Keep a stable
        // representation for forward-compatible finite decimals without locale
        // dependence.
        return java.math.BigDecimal.valueOf(value).stripTrailingZeros().toPlainString()
    }

    /** Match encoding/json string escaping with HTML escaping enabled (default). */
    private fun goJsonQuote(value: String): String {
        val out = StringBuilder(value.length + 2)
        out.append('\"')
        for (ch in value) {
            when (ch) {
                '\"' -> out.append("\\\"")
                '\\' -> out.append("\\\\")
                '\b' -> out.append("\\b")
                '\u000c' -> out.append("\\f")
                '\n' -> out.append("\\n")
                '\r' -> out.append("\\r")
                '\t' -> out.append("\\t")
                '<' -> out.append("\\u003c")
                '>' -> out.append("\\u003e")
                '&' -> out.append("\\u0026")
                '\u2028' -> out.append("\\u2028")
                '\u2029' -> out.append("\\u2029")
                else -> {
                    if (ch.code < 0x20) {
                        out.append("\\u00")
                        out.append(HEX_DIGITS[(ch.code ushr 4) and 0x0f])
                        out.append(HEX_DIGITS[ch.code and 0x0f])
                    } else {
                        out.append(ch)
                    }
                }
            }
        }
        out.append('\"')
        return out.toString()
    }

    private fun computeContentRootHash(
        entries: List<IntegrityFileEntry>,
        manifestHash: String,
        manifestBytes: Long,
    ): String {
        val merged = ArrayList<IntegrityFileEntry>(entries.size + 1)
        merged.addAll(entries)
        merged.add(IntegrityFileEntry(MANIFEST_PSEUDO_ENTRY_PATH, manifestHash, manifestBytes))
        val digest = MessageDigest.getInstance("SHA-256")
        for (entry in merged.sortedWith { left, right ->
            compareUtf8Strings(left.path, right.path)
        }) {
            digest.update("file".toByteArray(Charsets.UTF_8))
            digest.update(0.toByte())
            digest.update(entry.path.toByteArray(Charsets.UTF_8))
            digest.update(0.toByte())
            digest.update(entry.bytes.toString().toByteArray(Charsets.UTF_8))
            digest.update(0.toByte())
            digest.update(hexToBytes(entry.sha256))
            digest.update(0.toByte())
        }
        return digest.digest().joinToString("") { byte -> "%02x".format(byte) }
    }

    private fun hexToBytes(value: String): ByteArray {
        require(isLowerHexSha256(value)) { "invalid SHA-256 hex" }
        return ByteArray(value.length / 2) { index ->
            value.substring(index * 2, index * 2 + 2).toInt(16).toByte()
        }
    }

    /** Go strings compare lexicographically by UTF-8 bytes, not UTF-16 code units. */
    private fun compareUtf8Strings(left: String, right: String): Int {
        val a = left.toByteArray(Charsets.UTF_8)
        val b = right.toByteArray(Charsets.UTF_8)
        val limit = minOf(a.size, b.size)
        for (index in 0 until limit) {
            val av = a[index].toInt() and 0xff
            val bv = b[index].toInt() and 0xff
            if (av != bv) return av.compareTo(bv)
        }
        return a.size.compareTo(b.size)
    }

    /**
     * Compare the installed manifest with the DB-backed release manifest as JSON
     * values, not serialized text. This avoids reimplementing Go encoding/json
     * escaping rules on Android while still detecting any local semantic change,
     * including added/removed keys, reordered array elements, and changed numbers.
     */
    private fun jsonStructurallyEqual(left: Any?, right: Any?): Boolean {
        if (left === JSONObject.NULL || left == null) {
            return right === JSONObject.NULL || right == null
        }
        if (right === JSONObject.NULL || right == null) return false
        return when {
            left is JSONObject && right is JSONObject -> {
                val leftKeys = left.keys().asSequence().toSet()
                val rightKeys = right.keys().asSequence().toSet()
                leftKeys == rightKeys && leftKeys.all { key ->
                    jsonStructurallyEqual(left.opt(key), right.opt(key))
                }
            }
            left is JSONArray && right is JSONArray -> {
                left.length() == right.length() &&
                    (0 until left.length()).all { index ->
                        jsonStructurallyEqual(left.opt(index), right.opt(index))
                    }
            }
            left is Number && right is Number -> left.toString().toBigDecimalOrNull() == right.toString().toBigDecimalOrNull()
            else -> left == right
        }
    }

    private fun loadAction(installRoot: File, expectedKey: String, configPath: String): ActionSpec {
        val configFile = resolveUnder(installRoot, configPath)
        require(configFile.isFile) { "action config not found: $configPath" }
        val bytes = configFile.readBytes()
        val json = JSONObject(bytes.toString(Charsets.UTF_8))
        require(json.optInt("schemaVersion", 0) == PACKAGE_SCHEMA_VERSION) {
            "unsupported action schemaVersion for $expectedKey"
        }
        val key = json.optString("actionKey").trim()
        require(key == expectedKey) { "action key mismatch: expected=$expectedKey actual=$key" }
        val playbackMode = json.optString("playbackMode").trim().lowercase()
        require(playbackMode in SUPPORTED_PLAYBACK_MODES) {
            "unsupported playbackMode for $key: $playbackMode"
        }
        val fps = json.optInt("fps", 0)
        require(fps in 1..120) { "invalid fps for $key: $fps" }
        val fallbackDuration = (1000.0 / fps.toDouble()).roundToInt().toLong().coerceAtLeast(MIN_FRAME_DURATION_MS)
        val framesJson = json.optJSONArray("frames") ?: error("action $key has no frames")
        val frames = ArrayList<FrameSpec>(framesJson.length())
        val actionDir = configFile.parentFile ?: installRoot
        for (index in 0 until framesJson.length()) {
            val entry = framesJson.optJSONObject(index) ?: error("action $key frame $index is invalid")
            require(entry.optInt("index", -1) == index) { "action $key frame index mismatch at $index" }
            val fileName = entry.optString("file").trim()
            require(fileName.isNotEmpty()) { "action $key frame $index has no file" }
            val frameFile = resolveUnder(actionDir, fileName)
            require(frameFile.isFile) { "action frame not found: $fileName" }
            require(isDecodableImage(frameFile)) { "action frame cannot be decoded: $fileName" }
            val contentHash = entry.optString("contentHash").trim()
            if (contentHash.isNotEmpty()) {
                require(hashesEqual(contentHash, sha256Hex(frameFile))) {
                    "action frame contentHash mismatch: $fileName"
                }
            }
            val durationMs = entry.optLong("durationMs", fallbackDuration)
                .coerceIn(MIN_FRAME_DURATION_MS, MAX_FRAME_DURATION_MS)
            frames.add(FrameSpec(frameFile, durationMs))
        }
        require(frames.isNotEmpty()) { "action $key has no usable frames" }
        val returnJson = json.optJSONObject("returnTo")
        val returnType = returnJson?.optString("type")?.trim()?.lowercase().orEmpty().ifBlank { "default" }
        require(returnType in SUPPORTED_RETURN_TYPES) { "unsupported returnTo.type for $key: $returnType" }
        val returnAction = returnJson?.optString("actionKey")?.trim().orEmpty()
        if (returnType == "action") {
            require(returnAction.isNotEmpty()) { "returnTo.actionKey is required for $key" }
        }
        val interruptAfterMs = json.optLong("interruptAfterMs", 0L).coerceAtLeast(0L)
        val minimumPlayMs = json.optLong("minimumPlayMs", 0L).coerceAtLeast(0L)
        val maximumPlayMs: Long? = when {
            !json.has("maximumPlayMs") || json.isNull("maximumPlayMs") -> null
            else -> json.optLong("maximumPlayMs", 0L).coerceAtLeast(0L)
        }
        require(maximumPlayMs == null || maximumPlayMs >= minimumPlayMs) {
            "maximumPlayMs must be >= minimumPlayMs for $key"
        }
        require(maximumPlayMs == null || interruptAfterMs <= maximumPlayMs) {
            "interruptAfterMs must be <= maximumPlayMs for $key"
        }
        return ActionSpec(
            key = key,
            frames = frames,
            playbackMode = playbackMode,
            returnTo = ReturnRule(returnType, returnAction),
            interruptible = json.optBoolean("interruptible", true),
            interruptAfterMs = interruptAfterMs,
            minimumPlayMs = minimumPlayMs,
            maximumPlayMs = maximumPlayMs,
        )
    }

    private fun startAction(
        actionKey: String,
        requestedPlaybackRate: Double,
        forceLoop: Boolean?,
        clearTerminal: Boolean,
        requestedInterruptible: Boolean? = null,
    ): Map<String, Any?> {
        val pet = loadedPet ?: error("desktop pet renderer is not loaded")
        val action = pet.actions[actionKey] ?: error("action $actionKey is not available")
        ensureFirstFrameDecodable(action)
        stopAnimation()
        val priorActionKey = currentActionKey
        if (priorActionKey.isNotEmpty() && priorActionKey != action.key) {
            previousActionKey = priorActionKey
        }
        currentActionKey = action.key
        playbackId = "pb_android_${UUID.randomUUID()}"
        playbackStartedAt = System.currentTimeMillis()
        pauseStartedAt = 0L
        pausedAccumulatedMs = 0L
        frameIndex = 0
        cycleIndex = 0
        paused = false
        playbackRate = requestedPlaybackRate.coerceIn(0.25, 4.0)
        currentPlaybackMode = action.playbackMode
        currentInterruptible = action.interruptible && requestedInterruptible != false
        playbackActive = true
        if (clearTerminal) {
            lastCompletedPlaybackId = ""
            lastCompletionReason = ""
            lastCompletedPlayedMs = 0L
            lastCompletedCycleIndex = 0
        }
        val loop = forceLoop ?: (action.playbackMode == "loop" || action.playbackMode == "ping_pong")
        scheduleFrame(action, loop, playbackId)
        return mapOf(
            "playbackId" to playbackId,
            "actionKey" to action.key,
            "playbackMode" to if (loop && action.playbackMode == "once") "loop" else action.playbackMode,
            "singleCycleDurationMs" to scaledCycleDuration(action),
            "interruptible" to currentInterruptible,
            "interruptAfterMs" to action.interruptAfterMs,
            "minimumPlayMs" to action.minimumPlayMs,
            "maximumPlayMs" to action.maximumPlayMs,
            "returnTo" to mapOf(
                "type" to action.returnTo.type,
                if (action.returnTo.actionKey.isNotEmpty()) "actionKey" to action.returnTo.actionKey else "actionKey" to "",
            ),
        )
    }

    /**
     * Render first, then wait that frame's full duration before advancing.
     * Natural completion therefore happens only after the final frame has been
     * visible for its declared duration.
     */
    private fun scheduleFrame(action: ActionSpec, loop: Boolean, expectedPlaybackId: String) {
        val view = imageView ?: return
        val order = frameOrder(action)
        val runnable = object : Runnable {
            override fun run() {
                if (animationRunnable !== this || playbackId != expectedPlaybackId || currentActionKey != action.key) return
                if (paused) {
                    mainHandler.postDelayed(this, PAUSE_POLL_MS)
                    return
                }
                if (order.isEmpty()) return
                if (frameIndex >= order.size) {
                    cycleIndex++
                    when {
                        loop -> frameIndex = 0
                        action.playbackMode == "hold" -> {
                            animationRunnable = null
                            frameIndex = order.lastIndex
                            lastCompletedPlaybackId = expectedPlaybackId
                            lastCompletionReason = "natural_end"
                            lastCompletedPlayedMs = playedMs()
                            lastCompletedCycleIndex = cycleIndex.coerceAtLeast(1)
                            playbackActive = false
                            return
                        }
                        else -> {
                            animationRunnable = null
                            completeNaturalPlayback(action, expectedPlaybackId)
                            return
                        }
                    }
                }

                val sourceIndex = order[frameIndex.coerceIn(0, order.lastIndex)]
                val frame = action.frames[sourceIndex]
                decodeFrame(frame.file)?.let(view::setImageBitmap)
                val delay = (frame.durationMs / playbackRate)
                    .roundToInt()
                    .toLong()
                    .coerceAtLeast(MIN_FRAME_DURATION_MS)
                frameIndex++
                mainHandler.postDelayed(this, delay)
            }
        }
        animationRunnable = runnable
        // execute() runs on Dispatchers.Main.immediate. Render the first frame
        // before returning so Runtime V2 action_started reflects a physical
        // first-frame fact rather than mere command submission.
        runnable.run()
    }

    private fun ensureFirstFrameDecodable(action: ActionSpec) {
        val first = action.frames.firstOrNull() ?: error("desktop pet action has no frames: ${action.key}")
        require(decodeFrame(first.file) != null) {
            "desktop pet first frame cannot be decoded: ${first.file.name}"
        }
    }

    private fun isDecodableImage(file: File): Boolean {
        val options = BitmapFactory.Options().apply { inJustDecodeBounds = true }
        BitmapFactory.decodeFile(file.absolutePath, options)
        return options.outWidth > 0 && options.outHeight > 0
    }

    private fun completeNaturalPlayback(action: ActionSpec, completedPlaybackId: String) {
        if (playbackId != completedPlaybackId) return
        val duration = playedMs()
        lastCompletedPlaybackId = completedPlaybackId
        lastCompletionReason = "natural_end"
        lastCompletedPlayedMs = duration
        lastCompletedCycleIndex = cycleIndex.coerceAtLeast(1)
        playbackActive = false

        val pet = loadedPet ?: return
        val returnKey = when (action.returnTo.type) {
            "action" -> action.returnTo.actionKey.takeIf { pet.actions.containsKey(it) }
            "none" -> null
            "previous" -> previousActionKey.takeIf { pet.actions.containsKey(it) } ?: pet.defaultActionKey
            "current_activity", "default" -> pet.defaultActionKey
            else -> pet.defaultActionKey
        }
        if (!returnKey.isNullOrEmpty() && returnKey != action.key) {
            mainHandler.post {
                if (loadedPet === pet && playbackId == completedPlaybackId) {
                    startAction(
                        actionKey = returnKey,
                        requestedPlaybackRate = 1.0,
                        forceLoop = isLoopingDefaultAction(pet, returnKey),
                        clearTerminal = false,
                    )
                }
            }
        }
    }

    private fun frameOrder(action: ActionSpec): List<Int> {
        if (action.frames.size <= 1 || action.playbackMode != "ping_pong") {
            return action.frames.indices.toList()
        }
        val order = ArrayList<Int>(action.frames.size * 2 - 2)
        for (index in action.frames.indices) order.add(index)
        for (index in action.frames.lastIndex - 1 downTo 1) order.add(index)
        return order
    }

    private fun scaledCycleDuration(action: ActionSpec): Long {
        val order = frameOrder(action)
        return order.sumOf { index ->
            (action.frames[index].durationMs / playbackRate)
                .roundToInt()
                .toLong()
                .coerceAtLeast(MIN_FRAME_DURATION_MS)
        }.coerceAtLeast(1L)
    }

    private fun isLoopingDefaultAction(pet: LoadedPet, actionKey: String): Boolean {
        if (actionKey != pet.defaultActionKey) return false
        val mode = pet.actions[actionKey]?.playbackMode ?: return false
        return mode == "loop" || mode == "ping_pong"
    }

    private fun stopAnimation() {
        animationRunnable?.let { mainHandler.removeCallbacks(it) }
        animationRunnable = null
    }

    private fun resetPlaybackState(clearTerminal: Boolean) {
        currentActionKey = ""
        previousActionKey = ""
        playbackId = ""
        playbackStartedAt = 0L
        pauseStartedAt = 0L
        pausedAccumulatedMs = 0L
        frameIndex = 0
        cycleIndex = 0
        playbackRate = 1.0
        currentPlaybackMode = ""
        currentInterruptible = true
        playbackActive = false
        if (clearTerminal) {
            lastCompletedPlaybackId = ""
            lastCompletionReason = ""
            lastCompletedPlayedMs = 0L
            lastCompletedCycleIndex = 0
        }
    }

    private fun removeSurface() {
        val view = imageView
        imageView = null
        layoutParams = null
        visible = false
        if (view != null) {
            try {
                windowManager.removeViewImmediate(view)
            } catch (_: Exception) {
            }
        }
    }

    private fun buildLayoutParams(widthDp: Int, heightDp: Int, xDp: Int, yDp: Int): WindowManager.LayoutParams {
        val type = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            WindowManager.LayoutParams.TYPE_APPLICATION_OVERLAY
        } else {
            @Suppress("DEPRECATION")
            WindowManager.LayoutParams.TYPE_PHONE
        }
        return WindowManager.LayoutParams(
            dp(widthDp),
            dp(heightDp),
            type,
            WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE or
                WindowManager.LayoutParams.FLAG_LAYOUT_NO_LIMITS or
                WindowManager.LayoutParams.FLAG_LAYOUT_IN_SCREEN,
            PixelFormat.TRANSLUCENT,
        ).apply {
            gravity = Gravity.TOP or Gravity.START
            x = dp(xDp)
            y = dp(yDp)
        }
    }

    private fun installDragHandler(view: ImageView, params: WindowManager.LayoutParams) {
        val touchSlop = ViewConfiguration.get(appContext).scaledTouchSlop
        view.setOnTouchListener(object : View.OnTouchListener {
            private var downRawX = 0f
            private var downRawY = 0f
            private var downLocalX = 0f
            private var downLocalY = 0f
            private var startX = 0
            private var startY = 0
            private var dragging = false
            private var dragId = ""

            override fun onTouch(v: View, event: MotionEvent): Boolean {
                when (event.actionMasked) {
                    MotionEvent.ACTION_DOWN -> {
                        downRawX = event.rawX
                        downRawY = event.rawY
                        downLocalX = event.x
                        downLocalY = event.y
                        startX = params.x
                        startY = params.y
                        dragging = false
                        dragId = "drag_android_${UUID.randomUUID()}"
                        return true
                    }
                    MotionEvent.ACTION_MOVE -> {
                        val dx = (event.rawX - downRawX).roundToInt()
                        val dy = (event.rawY - downRawY).roundToInt()
                        if (!dragging && (kotlin.math.abs(dx) >= touchSlop || kotlin.math.abs(dy) >= touchSlop)) {
                            dragging = true
                            enqueueInteraction(
                                "runtime.drag.started",
                                mapOf(
                                    "dragId" to dragId,
                                    "startX" to pxToDp(startX),
                                    "startY" to pxToDp(startY),
                                    "currentX" to pxToDp(startX),
                                    "currentY" to pxToDp(startY),
                                    "occurredAtMs" to System.currentTimeMillis(),
                                ),
                            )
                        }
                        if (!dragging) return true
                        params.gravity = Gravity.TOP or Gravity.START
                        params.x = (startX + dx).coerceAtLeast(0)
                        params.y = (startY + dy).coerceAtLeast(0)
                        return try {
                            windowManager.updateViewLayout(view, params)
                            true
                        } catch (_: Exception) {
                            false
                        }
                    }
                    MotionEvent.ACTION_UP -> {
                        if (dragging) {
                            positionRevision++
                            enqueueInteraction(
                                "runtime.drag.completed",
                                mapOf(
                                    "dragId" to dragId,
                                    "startX" to pxToDp(startX),
                                    "startY" to pxToDp(startY),
                                    "currentX" to pxToDp(params.x),
                                    "currentY" to pxToDp(params.y),
                                    "positionRevision" to positionRevision,
                                    "occurredAtMs" to System.currentTimeMillis(),
                                ),
                            )
                        } else {
                            val canvas = canvasPoint(downLocalX, downLocalY, view)
                            enqueueInteraction(
                                "runtime.pointer.clicked",
                                mapOf(
                                    "canvasX" to canvas.first,
                                    "canvasY" to canvas.second,
                                    "screenX" to event.rawX.roundToInt(),
                                    "screenY" to event.rawY.roundToInt(),
                                    "occurredAtMs" to System.currentTimeMillis(),
                                ),
                            )
                        }
                        dragging = false
                        return true
                    }
                    MotionEvent.ACTION_CANCEL -> {
                        if (dragging) {
                            enqueueInteraction(
                                "runtime.drag.cancelled",
                                mapOf(
                                    "dragId" to dragId,
                                    "occurredAtMs" to System.currentTimeMillis(),
                                ),
                            )
                        }
                        dragging = false
                        return true
                    }
                }
                return false
            }
        })
    }

    private fun enqueueInteraction(type: String, payload: Map<String, Any?>) {
        while (interactionEvents.size >= MAX_INTERACTION_EVENTS) {
            interactionEvents.removeFirst()
        }
        interactionEvents.addLast(mapOf("type" to type, "payload" to payload))
    }

    private fun canvasPoint(localX: Float, localY: Float, view: ImageView): Pair<Int, Int> {
        val pet = loadedPet ?: return 0 to 0
        val viewWidth = view.width.coerceAtLeast(1).toFloat()
        val viewHeight = view.height.coerceAtLeast(1).toFloat()
        val canvasWidth = pet.canvasWidth.coerceAtLeast(1).toFloat()
        val canvasHeight = pet.canvasHeight.coerceAtLeast(1).toFloat()
        val scale = minOf(viewWidth / canvasWidth, viewHeight / canvasHeight)
        val renderedWidth = canvasWidth * scale
        val renderedHeight = canvasHeight * scale
        val offsetX = (viewWidth - renderedWidth) / 2f
        val offsetY = (viewHeight - renderedHeight) / 2f
        val x = ((localX - offsetX) / scale).coerceIn(0f, canvasWidth)
        val y = ((localY - offsetY) / scale).coerceIn(0f, canvasHeight)
        return x.roundToInt() to y.roundToInt()
    }

    private fun resolveDataPath(relativePath: String): File = resolveUnder(dataRoot, relativePath)

    private fun resolveUnder(root: File, relativePath: String): File {
        require(relativePath.isNotBlank()) { "path is empty" }
        require(!relativePath.startsWith("/") && !relativePath.contains('\\') && !relativePath.contains(':')) {
            "unsafe package path: $relativePath"
        }
        val canonicalRoot = root.canonicalFile
        val candidate = File(canonicalRoot, relativePath).canonicalFile
        require(candidate.path == canonicalRoot.path || candidate.path.startsWith(canonicalRoot.path + File.separator)) {
            "package path escapes runtime data root: $relativePath"
        }
        return candidate
    }

    private fun stateMap(): Map<String, Any?> {
        val pet = loadedPet
        val params = layoutParams
        return mapOf(
            "loaded" to (pet != null),
            "visible" to visible,
            "paused" to paused,
            "installationId" to (pet?.installationId ?: ""),
            "characterId" to (pet?.characterId ?: ""),
            "petId" to (pet?.petId ?: ""),
            "releaseId" to (pet?.releaseId ?: ""),
            "releaseVersion" to (pet?.releaseVersion ?: ""),
            "manifestHash" to (pet?.manifestHash ?: ""),
            "contentRootHash" to (pet?.contentRootHash ?: ""),
            "defaultActionKey" to (pet?.defaultActionKey ?: ""),
            "currentActionKey" to currentActionKey,
            "playbackId" to playbackId,
            "playbackActive" to playbackActive,
            "playbackMode" to currentPlaybackMode,
            "interruptible" to currentInterruptible,
            "playedMs" to playedMs(),
            "cycleIndex" to cycleIndex,
            "frameIndex" to frameIndex,
            "lastCompletedPlaybackId" to lastCompletedPlaybackId,
            "lastCompletionReason" to lastCompletionReason,
            "lastCompletedPlayedMs" to lastCompletedPlayedMs,
            "lastCompletedCycleIndex" to lastCompletedCycleIndex,
            "x" to pxToDp(params?.x ?: 0),
            "y" to pxToDp(params?.y ?: 0),
            "width" to pxToDp(params?.width ?: 0),
            "height" to pxToDp(params?.height ?: 0),
            "scale" to if (pet != null && params != null && pet.canvasWidth > 0) {
                pxToDp(params.width).toDouble() / pet.canvasWidth.toDouble()
            } else {
                1.0
            },
            "alpha" to alpha,
            "positionRevision" to positionRevision,
        )
    }

    private fun playedMs(): Long = if (playbackStartedAt <= 0L) {
        0L
    } else {
        val now = System.currentTimeMillis()
        val currentPauseMs = if (paused && pauseStartedAt > 0L) {
            (now - pauseStartedAt).coerceAtLeast(0L)
        } else {
            0L
        }
        (now - playbackStartedAt - pausedAccumulatedMs - currentPauseMs).coerceAtLeast(0L)
    }

    private fun decodeFrame(file: File): Bitmap? {
        val key = file.absolutePath
        bitmapCache.get(key)?.let { return it }
        return try {
            BitmapFactory.decodeFile(key)?.also { bitmapCache.put(key, it) }
        } catch (_: OutOfMemoryError) {
            bitmapCache.evictAll()
            null
        }
    }

    private fun overlayPermissionGranted(): Boolean =
        Build.VERSION.SDK_INT < Build.VERSION_CODES.M || Settings.canDrawOverlays(appContext)

    private fun dp(value: Int): Int = (value * appContext.resources.displayMetrics.density).roundToInt()
    private fun pxToDp(value: Int): Int = (value / appContext.resources.displayMetrics.density).roundToInt()

    private fun sha256Hex(bytes: ByteArray): String =
        MessageDigest.getInstance("SHA-256")
            .digest(bytes)
            .joinToString("") { byte -> "%02x".format(byte) }

    private fun sha256Hex(file: File): String {
        val digest = MessageDigest.getInstance("SHA-256")
        file.inputStream().use { input ->
            val buffer = ByteArray(DEFAULT_HASH_BUFFER_BYTES)
            while (true) {
                val read = input.read(buffer)
                if (read <= 0) break
                digest.update(buffer, 0, read)
            }
        }
        return digest.digest().joinToString("") { byte -> "%02x".format(byte) }
    }

    private fun isLowerHexSha256(value: String): Boolean =
        value.length == 64 && value == value.lowercase() && SHA256_RE.matches(value)

    private fun isSemVer(value: String): Boolean = SEMVER_RE.matches(value.trim())

    private fun compareSemVer(left: String, right: String): Int {
        val l = SEMVER_RE.matchEntire(left.trim()) ?: error("invalid semver: $left")
        val r = SEMVER_RE.matchEntire(right.trim()) ?: error("invalid semver: $right")
        for (index in 1..3) {
            val cmp = l.groupValues[index].toLong().compareTo(r.groupValues[index].toLong())
            if (cmp != 0) return cmp
        }
        // A prerelease is lower precedence than the corresponding release. The
        // exact identifier ordering is irrelevant for our current 2.0.0 runtime
        // floor/ceiling, but preserving this rule avoids accepting a prerelease
        // above a stable boundary.
        val lPre = l.groupValues[4]
        val rPre = r.groupValues[4]
        if (lPre.isEmpty() && rPre.isNotEmpty()) return 1
        if (lPre.isNotEmpty() && rPre.isEmpty()) return -1
        return lPre.compareTo(rPre)
    }

    private fun hashesEqual(expected: String, actual: String): Boolean {
        val normalizedExpected = expected.trim().lowercase().removePrefix("sha256:")
        val normalizedActual = actual.trim().lowercase().removePrefix("sha256:")
        return normalizedExpected.isNotEmpty() && normalizedExpected == normalizedActual
    }

    private fun success(request: NativeBridgeRequest, result: Map<String, Any?>): NativeBridgeResponse = NativeBridgeResponse(
        protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
        requestId = request.requestId,
        status = NativeBridgeProtocol.STATUS_SUCCESS,
        result = result,
    )

    private fun failure(
        request: NativeBridgeRequest,
        code: String,
        message: String,
        domainCode: String? = null,
    ): NativeBridgeResponse = NativeBridgeResponse(
        protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
        requestId = request.requestId,
        status = NativeBridgeProtocol.STATUS_ERROR,
        error = NativeBridgeError(code = code, message = message, domainCode = domainCode),
    )

    private fun Map<String, Any?>.string(key: String): String = this[key]?.toString()?.trim().orEmpty()
    private fun Map<String, Any?>.int(key: String, fallback: Int): Int = intOrNull(key) ?: fallback
    private fun Map<String, Any?>.intOrNull(key: String): Int? = when (val value = this[key]) {
        is Number -> value.toInt()
        is String -> value.toIntOrNull()
        else -> null
    }
    private fun Map<String, Any?>.float(key: String, fallback: Float): Float = floatOrNull(key) ?: fallback
    private fun Map<String, Any?>.floatOrNull(key: String): Float? = when (val value = this[key]) {
        is Number -> value.toFloat()
        is String -> value.toFloatOrNull()
        else -> null
    }
    private fun Map<String, Any?>.double(key: String, fallback: Double): Double = when (val value = this[key]) {
        is Number -> value.toDouble()
        is String -> value.toDoubleOrNull() ?: fallback
        else -> fallback
    }
    private fun Map<String, Any?>.booleanOrNull(key: String): Boolean? = when (val value = this[key]) {
        is Boolean -> value
        is Number -> value.toInt() != 0
        is String -> when (value.trim().lowercase()) {
            "true", "1" -> true
            "false", "0" -> false
            else -> null
        }
        else -> null
    }

    companion object {
        private const val PACKAGE_SCHEMA_VERSION = 2
        private const val PACKAGE_MANIFEST_FORMAT = "amitia-desktop-pet"
        private const val INTEGRITY_ALGORITHM = "amitia-package-sha256-v2"
        private const val MANIFEST_PSEUDO_ENTRY_PATH = "@manifest"
        private const val RUNTIME_VERSION = "2.0.0"
        private const val MAX_CANVAS_PX = 4096
        private const val DEFAULT_HASH_BUFFER_BYTES = 64 * 1024
        private const val MIN_FRAME_DURATION_MS = 8L
        private const val MAX_FRAME_DURATION_MS = 60_000L
        private const val PAUSE_POLL_MS = 50L
        private const val DEFAULT_SIZE_DP = 180
        private const val MIN_PET_DP = 64
        private const val MAX_PET_DP = 420
        private const val DEFAULT_X_DP = 16
        private const val DEFAULT_Y_DP = 120
        private const val BITMAP_CACHE_KB = 24 * 1024
        private const val MAX_INTERACTION_EVENTS = 128
        private val SUPPORTED_PLAYBACK_MODES = setOf("loop", "once", "hold", "ping_pong")
        private val SUPPORTED_RETURN_TYPES = setOf("default", "previous", "current_activity", "none", "action")
        private val SUPPORTED_BINDING_POLICIES = setOf("bound", "unbound", "legacy_inferred")
        private const val HEX_DIGITS = "0123456789abcdef"
        private val SHA256_RE = Regex("^[0-9a-f]{64}$")
        private val SEMVER_RE = Regex("^(\\d+)\\.(\\d+)\\.(\\d+)(?:-([0-9A-Za-z.-]+))?(?:\\+[0-9A-Za-z.-]+)?$")

        const val OP_STATUS = "desktop.pet.renderer.status"
        const val OP_LOAD = "desktop.pet.renderer.load"
        const val OP_UNLOAD = "desktop.pet.renderer.unload"
        const val OP_SHOW = "desktop.pet.renderer.show"
        const val OP_HIDE = "desktop.pet.renderer.hide"
        const val OP_SETTINGS = "desktop.pet.renderer.settings"
        const val OP_PLAY = "desktop.pet.renderer.play"
        const val OP_STOP = "desktop.pet.renderer.stop"
        const val OP_PAUSE = "desktop.pet.renderer.pause"
        const val OP_RESUME = "desktop.pet.renderer.resume"
        const val OP_RECENTER = "desktop.pet.renderer.recenter"
        const val OP_DRAIN_EVENTS = "desktop.pet.renderer.events.drain"
    }
}
