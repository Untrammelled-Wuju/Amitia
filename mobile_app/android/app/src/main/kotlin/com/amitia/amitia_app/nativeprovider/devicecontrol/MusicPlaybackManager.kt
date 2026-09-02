package com.amitia.amitia_app.nativeprovider.devicecontrol

import android.content.Context
import android.media.MediaPlayer
import android.net.Uri
import java.util.concurrent.Executors
import kotlin.math.max

/**
 * Small Amitia-owned audio player used by structured Android model tools.
 *
 * It intentionally does not attempt to seize another application's media
 * session. Sources and queue entries are explicit, bounded by the Go tool
 * schema, and playback state is kept inside this process.
 */
internal class MusicPlaybackManager(
    private val context: Context,
) {
    private val lock = Any()
    private val completionExecutor = Executors.newSingleThreadExecutor { runnable ->
        Thread(runnable, "amitia-music-next").apply { isDaemon = true }
    }

    private var player: MediaPlayer? = null
    private var queue: List<String> = emptyList()
    private var queueIndex: Int = -1
    private var volume: Float = 1f
    private var state: String = STATE_IDLE
    private var lastPositionMs: Int = 0
    private var lastDurationMs: Int = 0
    private var lastError: String? = null

    fun play(source: String, startPositionMs: Int = 0): Map<String, Any?> = synchronized(lock) {
        require(source.isNotBlank()) { "source is required" }
        queue = listOf(source.trim())
        queueIndex = 0
        startCurrentLocked(startPositionMs.coerceAtLeast(0))
        statusLocked()
    }

    fun playQueue(sources: List<String>, startIndex: Int = 0): Map<String, Any?> = synchronized(lock) {
        val normalized = sources.map { it.trim() }.filter { it.isNotEmpty() }
        require(normalized.isNotEmpty()) { "sources must not be empty" }
        require(normalized.size <= MAX_QUEUE_SIZE) { "sources exceeds $MAX_QUEUE_SIZE entries" }
        require(startIndex in normalized.indices) { "startIndex is out of range" }
        queue = normalized
        queueIndex = startIndex
        startCurrentLocked(0)
        statusLocked()
    }

    fun pause(): Map<String, Any?> = synchronized(lock) {
        val current = player ?: throw IllegalStateException("no active media source")
        if (state == STATE_PLAYING && current.isPlaying) {
            lastPositionMs = current.currentPosition.coerceAtLeast(0)
            current.pause()
            state = STATE_PAUSED
        }
        statusLocked()
    }

    fun resume(): Map<String, Any?> = synchronized(lock) {
        if (queueIndex !in queue.indices) throw IllegalStateException("no media source is queued")
        val current = player
        if (current == null || state == STATE_STOPPED || state == STATE_COMPLETED || state == STATE_ERROR) {
            startCurrentLocked(lastPositionMs.coerceAtLeast(0))
        } else if (state != STATE_PLAYING) {
            current.start()
            state = STATE_PLAYING
        }
        statusLocked()
    }

    fun stop(): Map<String, Any?> = synchronized(lock) {
        player?.let { current ->
            lastPositionMs = runCatching { current.currentPosition }.getOrDefault(lastPositionMs).coerceAtLeast(0)
        }
        releasePlayerLocked()
        state = if (queueIndex in queue.indices) STATE_STOPPED else STATE_IDLE
        statusLocked()
    }

    fun seek(positionMs: Int): Map<String, Any?> = synchronized(lock) {
        val current = player ?: throw IllegalStateException("no active media source")
        val bounded = if (lastDurationMs > 0) positionMs.coerceIn(0, lastDurationMs) else max(0, positionMs)
        current.seekTo(bounded)
        lastPositionMs = bounded
        statusLocked()
    }

    fun setVolume(value: Float): Map<String, Any?> = synchronized(lock) {
        volume = value.coerceIn(0f, 1f)
        player?.setVolume(volume, volume)
        statusLocked()
    }

    fun status(): Map<String, Any?> = synchronized(lock) { statusLocked() }

    private fun startCurrentLocked(startPositionMs: Int) {
        if (queueIndex !in queue.indices) throw IllegalStateException("queue index is invalid")
        releasePlayerLocked()
        state = STATE_PREPARING
        lastError = null
        lastPositionMs = 0
        lastDurationMs = 0
        val source = queue[queueIndex]
        val next = MediaPlayer()
        try {
            configureDataSource(next, source)
            next.setVolume(volume, volume)
            next.setOnCompletionListener {
                completionExecutor.execute { onCompleted() }
            }
            next.setOnErrorListener { _, what, extra ->
                synchronized(lock) {
                    lastError = "MediaPlayer error what=$what extra=$extra"
                    state = STATE_ERROR
                    releasePlayerLocked()
                }
                true
            }
            next.prepare()
            lastDurationMs = runCatching { next.duration }.getOrDefault(0).coerceAtLeast(0)
            val boundedStart = if (lastDurationMs > 0) startPositionMs.coerceIn(0, lastDurationMs) else startPositionMs
            if (boundedStart > 0) next.seekTo(boundedStart)
            lastPositionMs = boundedStart
            player = next
            next.start()
            state = STATE_PLAYING
        } catch (t: Throwable) {
            runCatching { next.release() }
            player = null
            state = STATE_ERROR
            lastError = t.message ?: t.javaClass.simpleName
            throw IllegalStateException("failed to play media source: ${lastError}", t)
        }
    }

    private fun configureDataSource(target: MediaPlayer, source: String) {
        val uri = runCatching { Uri.parse(source) }.getOrNull()
        val scheme = uri?.scheme?.lowercase()
        when (scheme) {
            "content", "android.resource", "file" -> target.setDataSource(context, uri!!)
            "http", "https" -> target.setDataSource(source)
            else -> throw IllegalArgumentException("unsupported media source scheme: ${scheme ?: "missing"}")
        }
    }

    private fun onCompleted() = synchronized(lock) {
        if (state == STATE_ERROR) return@synchronized
        lastPositionMs = lastDurationMs
        releasePlayerLocked()
        if (queueIndex + 1 < queue.size) {
            queueIndex += 1
            runCatching { startCurrentLocked(0) }.onFailure {
                lastError = it.message ?: it.javaClass.simpleName
                state = STATE_ERROR
            }
        } else {
            state = STATE_COMPLETED
        }
    }

    private fun releasePlayerLocked() {
        val current = player
        player = null
        if (current != null) {
            runCatching { current.setOnCompletionListener(null) }
            runCatching { current.setOnErrorListener(null) }
            runCatching { current.release() }
        }
    }

    private fun statusLocked(): Map<String, Any?> {
        val current = player
        val position = if (current != null && state in setOf(STATE_PLAYING, STATE_PAUSED)) {
            runCatching { current.currentPosition }.getOrDefault(lastPositionMs).coerceAtLeast(0)
        } else {
            lastPositionMs.coerceAtLeast(0)
        }
        val duration = if (current != null && state in setOf(STATE_PLAYING, STATE_PAUSED)) {
            runCatching { current.duration }.getOrDefault(lastDurationMs).coerceAtLeast(0)
        } else {
            lastDurationMs.coerceAtLeast(0)
        }
        lastPositionMs = position
        lastDurationMs = duration
        return linkedMapOf(
            "state" to state,
            "playing" to (state == STATE_PLAYING),
            "queue" to queue,
            "queueIndex" to queueIndex,
            "source" to queue.getOrNull(queueIndex),
            "positionMs" to position,
            "durationMs" to duration,
            "volume" to volume.toDouble(),
            "error" to lastError,
        )
    }

    companion object {
        private const val MAX_QUEUE_SIZE = 100
        private const val STATE_IDLE = "idle"
        private const val STATE_PREPARING = "preparing"
        private const val STATE_PLAYING = "playing"
        private const val STATE_PAUSED = "paused"
        private const val STATE_STOPPED = "stopped"
        private const val STATE_COMPLETED = "completed"
        private const val STATE_ERROR = "error"
    }
}
