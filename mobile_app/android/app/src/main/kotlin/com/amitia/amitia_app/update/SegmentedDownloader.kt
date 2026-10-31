package com.amitia.amitia_app.update

import java.io.File
import java.io.FileOutputStream
import java.net.HttpURLConnection
import java.net.URL
import java.security.MessageDigest
import java.util.Properties
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicLong

class RangeUnsupportedException(message: String) : Exception(message)

class SegmentedDownloader(
    private val chunkSize: Int = DEFAULT_CHUNK_SIZE,
    private val concurrency: Int = DEFAULT_CONCURRENCY,
    private val retries: Int = 3,
    private val connectTimeoutMs: Int = 15_000,
    private val readTimeoutMs: Int = 60_000,
) {
    fun download(
        url: String,
        target: File,
        expectedSize: Long,
        expectedSha256: String,
        onProgress: (downloaded: Long, total: Long) -> Unit = { _, _ -> },
        isCancelled: () -> Boolean = { false },
    ): File {
        require(expectedSize > 0)
        require(expectedSha256.matches(Regex("[a-fA-F0-9]{64}")))
        target.parentFile?.mkdirs()
        if (matchesFile(target, expectedSize, expectedSha256)) {
            onProgress(expectedSize, expectedSize)
            return target
        }
        return try {
            downloadRanged(
                url,
                target,
                expectedSize,
                expectedSha256.lowercase(),
                onProgress,
                isCancelled,
            )
        } catch (error: RangeUnsupportedException) {
          error.printStackTrace()

            downloadSingle(
                url,
                target,
                expectedSize,
                expectedSha256.lowercase(),
                onProgress,
                isCancelled,
            )
        }
    }

    private fun downloadRanged(
        url: String,
        target: File,
        expectedSize: Long,
        expectedSha256: String,
        onProgress: (Long, Long) -> Unit,
        isCancelled: () -> Boolean,
    ): File {
        val remote = probe(url, expectedSize)
        val count = ((remote.total + chunkSize - 1) / chunkSize).toInt()
        val workers = concurrency.coerceIn(1, 8).coerceAtMost(count)
        val stateDirectory = File(target.parentFile, ".${target.name}.segments")
        stateDirectory.mkdirs()
        prepareState(stateDirectory, url, remote, expectedSize, count, expectedSha256)
        val progress = Array(count) { 0L }
        val networkBytes = AtomicLong(0)
        val lastProgressAt = AtomicLong(0)
        val startedAt = System.currentTimeMillis()
        fun emitProgress(force: Boolean = false) {
            val now = System.currentTimeMillis()
            val previous = lastProgressAt.get()
            if (!force && now - previous < 300) return
            if (!lastProgressAt.compareAndSet(previous, now) && !force) return
            val downloaded = synchronized(progress) { progress.sum() }
            val elapsed = ((now - startedAt).coerceAtLeast(1)) / 1000.0
            onProgress(downloaded, expectedSize)
        }

        val executor = Executors.newFixedThreadPool(workers)
        val cancelled = AtomicBoolean(false)
        try {
            val futures = (0 until count).map { index ->
                executor.submit {
                    val expected = partLength(expectedSize, index)
                    val part = File(stateDirectory, partName(index))
                    if (part.isFile && part.length() == expected) {
                        synchronized(progress) { progress[index] = expected }
                        emitProgress()
                        return@submit
                    }
                    part.delete()
                    var lastError: Throwable? = null
                    for (attempt in 0..retries) {
                        if (isCancelled() || cancelled.get()) throw InterruptedException("download cancelled")
                        try {
                            downloadPart(
                                url,
                                remote.total,
                                index,
                                part,
                                expected,
                                onBytes = { bytes ->
                                    val old = synchronized(progress) {
                                        val previous = progress[index]
                                        progress[index] = bytes
                                        previous
                                    }
                                    if (bytes > old) networkBytes.addAndGet(bytes - old)
                                    emitProgress()
                                },
                                isCancelled = isCancelled,
                            )
                            return@submit
                        } catch (error: Throwable) {
                            lastError = error
                            synchronized(progress) { progress[index] = 0L }
                            if (error is RangeUnsupportedException || error is InterruptedException) throw error
                            if (attempt < retries) Thread.sleep(400L * (1 shl attempt))
                        }
                    }
                    throw lastError ?: IllegalStateException("download part failed")
                }
            }
            futures.forEach { future ->
                try {
                    future.get()
                } catch (error: Exception) {
                    cancelled.set(true)
                    throw error.cause ?: error
                }
            }
        } finally {
            executor.shutdownNow()
            executor.awaitTermination(5, TimeUnit.SECONDS)
        }
        emitProgress(true)
        val temporary = File(target.parentFile, "${target.name}.partial")
        val digest = MessageDigest.getInstance("SHA-256")
        var total = 0L
        FileOutputStream(temporary).use { output ->
            for (index in 0 until count) {
                val part = File(stateDirectory, partName(index))
                part.inputStream().use { input ->
                    val buffer = ByteArray(BUFFER_SIZE)
                    while (true) {
                        val read = input.read(buffer)
                        if (read <= 0) break
                        digest.update(buffer, 0, read)
                        output.write(buffer, 0, read)
                        total += read
                    }
                }
            }
            output.flush()
            output.fd.sync()
        }
        val actual = digest.digest().joinToString("") { "%02x".format(it) }
        if (total != expectedSize || actual != expectedSha256) {
            temporary.delete()
            stateDirectory.deleteRecursively()
            throw IllegalStateException("download integrity verification failed")
        }
        target.delete()
        if (!temporary.renameTo(target)) throw IllegalStateException("download file could not be finalized")
        stateDirectory.deleteRecursively()
        onProgress(expectedSize, expectedSize)
        return target
    }

    private fun downloadSingle(
        url: String,
        target: File,
        expectedSize: Long,
        expectedSha256: String,
        onProgress: (Long, Long) -> Unit,
        isCancelled: () -> Boolean,
    ): File {
        val temporary = File(target.parentFile, "${target.name}.single.partial")
        temporary.delete()
        val digest = MessageDigest.getInstance("SHA-256")
        var total = 0L
        var lastError: Throwable? = null
        for (attempt in 0..retries) {
            total = 0
            digest.reset()
            temporary.delete()
            try {
                val connection = openConnection(url)
                connection.setRequestProperty("Accept-Encoding", "identity")
                val status = connection.responseCode
                if (status !in 200..299) throw IllegalStateException("download request failed: HTTP $status")
                connection.inputStream.use { input ->
                    FileOutputStream(temporary).use { output ->
                        val buffer = ByteArray(BUFFER_SIZE)
                        while (true) {
                            if (isCancelled()) throw InterruptedException("download cancelled")
                            val read = input.read(buffer)
                            if (read <= 0) break
                            digest.update(buffer, 0, read)
                            output.write(buffer, 0, read)
                            total += read
                            onProgress(total, expectedSize)
                        }
                        output.flush()
                        output.fd.sync()
                    }
                }
                connection.disconnect()
                val actual = digest.digest().joinToString("") { "%02x".format(it) }
                if (total != expectedSize || actual != expectedSha256) {
                    throw IllegalStateException("download integrity verification failed")
                }
                target.delete()
                if (!temporary.renameTo(target)) throw IllegalStateException("download file could not be finalized")
                onProgress(expectedSize, expectedSize)
                return target
            } catch (error: Throwable) {
                lastError = error
                temporary.delete()
                if (error is InterruptedException || attempt == retries) break
                Thread.sleep(500L * (1 shl attempt))
            }
        }
        throw lastError ?: IllegalStateException("download failed")
    }

    private data class RemoteFile(
        val total: Long,
        val etag: String,
        val lastModified: String,
    )

    private fun probe(url: String, expectedSize: Long): RemoteFile {
        val connection = openConnection(url)
        connection.setRequestProperty("Range", "bytes=0-0")
        connection.setRequestProperty("Accept-Encoding", "identity")
        connection.setRequestProperty("Cache-Control", "no-cache")
        val status = connection.responseCode
        val contentRange = connection.getHeaderField("Content-Range").orEmpty()
        val match = Regex("^bytes\\s+\\d+-\\d+/(\\d+)$", RegexOption.IGNORE_CASE).find(contentRange)
        val total = match?.groupValues?.getOrNull(1)?.toLongOrNull() ?: 0L
        val supported = status == 206 && total > 0
        val remote = RemoteFile(
            total = total,
            etag = connection.getHeaderField("ETag").orEmpty(),
            lastModified = connection.getHeaderField("Last-Modified").orEmpty(),
        )
        runCatching { connection.inputStream?.close() }
        connection.disconnect()
        if (!supported) throw RangeUnsupportedException("server does not support byte ranges")
        if (expectedSize > 0 && total != expectedSize) {
            throw IllegalStateException("remote update size does not match the manifest")
        }
        return remote
    }

    private fun prepareState(
        directory: File,
        url: String,
        remote: RemoteFile,
        expectedSize: Long,
        count: Int,
        expectedSha256: String,
    ) {
        val metadata = File(directory, "state.properties")
        val values = Properties()
        if (metadata.isFile) runCatching { metadata.inputStream().use(values::load) }
        val matches =
            values.getProperty("url") == url &&
                values.getProperty("total")?.toLongOrNull() == expectedSize &&
                values.getProperty("etag") == remote.etag &&
                values.getProperty("lastModified") == remote.lastModified &&
                values.getProperty("chunkSize")?.toIntOrNull() == chunkSize &&
                values.getProperty("chunkCount")?.toIntOrNull() == count &&
                values.getProperty("sha256") == expectedSha256
        if (!matches) {
            directory.listFiles()?.forEach { file ->
                if (file.extension == "part") file.delete()
            }
            values.clear()
            values.setProperty("url", url)
            values.setProperty("total", expectedSize.toString())
            values.setProperty("etag", remote.etag)
            values.setProperty("lastModified", remote.lastModified)
            values.setProperty("chunkSize", chunkSize.toString())
            values.setProperty("chunkCount", count.toString())
            values.setProperty("sha256", expectedSha256)
            val temporary = File(directory, "state.properties.partial")
            temporary.outputStream().use { values.store(it, "") }
            metadata.delete()
            if (!temporary.renameTo(metadata)) throw IllegalStateException("download state could not be stored")
        }
    }

    private fun downloadPart(
        url: String,
        total: Long,
        index: Int,
        part: File,
        expected: Long,
        onBytes: (Long) -> Unit,
        isCancelled: () -> Boolean,
    ) {
        val start = index.toLong() * chunkSize
        val end = start + expected - 1
        val connection = openConnection(url)
        connection.setRequestProperty("Range", "bytes=$start-$end")
        connection.setRequestProperty("Accept-Encoding", "identity")
        connection.setRequestProperty("Cache-Control", "no-cache")
        val status = connection.responseCode
        if (status != 206) {
            connection.disconnect()
            throw RangeUnsupportedException("range request failed: HTTP $status")
        }
        val contentRange = connection.getHeaderField("Content-Range").orEmpty()
        if (!contentRange.endsWith("/$total")) {
            connection.disconnect()
            throw IllegalStateException("range response does not match remote file")
        }
        val temporary = File(part.parentFile, "${part.name}.partial")
        temporary.delete()
        var written = 0L
        try {
            connection.inputStream.use { input ->
                FileOutputStream(temporary).use { output ->
                    val buffer = ByteArray(BUFFER_SIZE)
                    while (true) {
                        if (isCancelled()) throw InterruptedException("download cancelled")
                        val read = input.read(buffer)
                        if (read <= 0) break
                        written += read
                        if (written > expected) throw IllegalStateException("download range exceeds expected size")
                        output.write(buffer, 0, read)
                        onBytes(written)
                    }
                    output.flush()
                    output.fd.sync()
                }
            }
            if (written != expected) throw IllegalStateException("download range is incomplete")
            if (!temporary.renameTo(part)) throw IllegalStateException("download range could not be stored")
        } catch (error: Throwable) {
            temporary.delete()
            throw error
        } finally {
            connection.disconnect()
        }
    }

    private fun openConnection(url: String): HttpURLConnection {
        return (URL(url).openConnection() as HttpURLConnection).apply {
            requestMethod = "GET"
            instanceFollowRedirects = true
            connectTimeout = connectTimeoutMs
            readTimeout = readTimeoutMs
            useCaches = false
        }
    }

    private fun partLength(total: Long, index: Int): Long {
        val start = index.toLong() * chunkSize
        return (total - start).coerceAtMost(chunkSize.toLong())
    }

    private fun partName(index: Int) = String.format("%08d.part", index)

    private fun matchesFile(file: File, size: Long, sha256: String): Boolean {
        if (!file.isFile || file.length() != size) return false
        val digest = MessageDigest.getInstance("SHA-256")
        file.inputStream().use { input ->
            val buffer = ByteArray(BUFFER_SIZE)
            while (true) {
                val read = input.read(buffer)
                if (read <= 0) break
                digest.update(buffer, 0, read)
            }
        }
        return digest.digest().joinToString("") { "%02x".format(it) }.equals(sha256, ignoreCase = true)
    }

    companion object {
        const val DEFAULT_CHUNK_SIZE = 8 * 1024 * 1024
        const val DEFAULT_CONCURRENCY = 4
        private const val BUFFER_SIZE = 128 * 1024
    }
}
