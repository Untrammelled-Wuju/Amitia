package com.amitia.amitia_app.update

import org.junit.After
import org.junit.Assume
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File
import java.io.RandomAccessFile
import java.net.InetAddress
import java.net.ServerSocket
import java.nio.file.Files
import java.security.MessageDigest
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicBoolean

class SegmentedDownloaderTest {
    private val servers = mutableListOf<AutoCloseable>()
    private val directories = mutableListOf<File>()

    @After
    fun tearDown() {
        servers.forEach { it.close() }
        directories.forEach { it.deleteRecursively() }
    }

    @Test
    fun downloadsTenConsecutiveFilesWithConcurrentRanges() {
        for (round in 0 until 10) {
            val content = ByteArray(900_000 + round * 37) { index ->
                ((index * 19 + round) % 251).toByte()
            }
            val active = AtomicInteger(0)
            val maximum = AtomicInteger(0)
            val fixture = serve(content, supportRanges = true) { range ->
                val running = active.incrementAndGet()
                maximum.accumulateAndGet(running) { left, right -> maxOf(left, right) }
                Thread.sleep(8)
                try {
                    range()
                } finally {
                    active.decrementAndGet()
                }
            }
            val directory = Files.createTempDirectory("amitia-android-download-").toFile()
            directories += directory
            val target = File(directory, "update.apk")
            val result = SegmentedDownloader(
                chunkSize = 128 * 1024,
                concurrency = 4,
                retries = 1,
            ).download(
                fixture,
                target,
                content.size.toLong(),
                sha256(content),
            )
            assertArrayEquals(content, result.readBytes())
            assertTrue("round $round did not use concurrent ranges", maximum.get() > 1)
        }
    }

    @Test
    fun fallsBackToSingleThreadWhenRangesAreUnsupported() {
        val content = "fallback-download".toByteArray()
        val directory = Files.createTempDirectory("amitia-android-fallback-").toFile()
        directories += directory
        val target = File(directory, "update.apk")
        val url = serve(content, supportRanges = false)
        val result = SegmentedDownloader(chunkSize = 4, concurrency = 2).download(
            url,
            target,
            content.size.toLong(),
            sha256(content),
        )
        assertArrayEquals(content, result.readBytes())
    }

    @Test
    fun downloadsTenProductionArtifactsWhenConfigured() {
        val url = System.getenv("AMITIA_ANDROID_UPDATE_URL").orEmpty()
        val artifactPath = System.getenv("AMITIA_ANDROID_UPDATE_ARTIFACT").orEmpty()
        val expectedSha256 = System.getenv("AMITIA_ANDROID_UPDATE_SHA256").orEmpty()
        val expectedSize = System.getenv("AMITIA_ANDROID_UPDATE_SIZE")?.toLongOrNull() ?: 0L
        val rounds = System.getenv("AMITIA_PRODUCTION_TEST_ROUNDS")?.toIntOrNull() ?: 10
        val remoteRounds =
            System.getenv("AMITIA_ANDROID_REMOTE_FULL_ROUNDS")?.toIntOrNull() ?: 0
        Assume.assumeTrue(
            artifactPath.isNotEmpty() &&
                expectedSha256.matches(Regex("[a-fA-F0-9]{64}")) &&
                expectedSize > 0,
        )
        val artifact = File(artifactPath)
        assertEquals(expectedSize, artifact.length())
        assertEquals(expectedSha256.lowercase(), sha256(artifact))
        if (url.isNotEmpty()) verifyProductionRanges(url, artifact, expectedSize)
        val rangeServer = FileRangeServer(artifact)
        servers += rangeServer
        val directory = Files.createTempDirectory("amitia-android-production-").toFile()
        directories += directory
        val downloader = SegmentedDownloader()
        repeat(remoteRounds) { round ->
            val target = File(directory, "remote-production-${round + 1}.apk")
            val startedAt = System.currentTimeMillis()
            val result = downloader.download(url, target, expectedSize, expectedSha256)
            assertEquals(expectedSize, result.length())
            assertEquals(expectedSha256.lowercase(), sha256(result))
            println(
                "[android-production-remote] round ${round + 1}/$remoteRounds pass " +
                    "${System.currentTimeMillis() - startedAt}ms",
            )
            target.delete()
        }
        repeat(rounds) { round ->
            val target = File(directory, "production-${round + 1}.apk")
            val startedAt = System.currentTimeMillis()
            val result = downloader.download(rangeServer.url, target, expectedSize, expectedSha256)
            assertEquals(expectedSize, result.length())
            assertEquals(expectedSha256.lowercase(), sha256(result))
            println(
                "[android-production] round ${round + 1}/$rounds pass " +
                    "${System.currentTimeMillis() - startedAt}ms",
            )
            target.delete()
        }
    }

    private fun serve(
        content: ByteArray,
        supportRanges: Boolean,
        delay: ((() -> Unit) -> Unit)? = null,
    ): String {
        val server = TestHttpServer(content, supportRanges, delay)
        servers += server
        return server.url
    }

    private fun parseRange(value: String?, total: Int): Pair<Int, Int>? {
        val match = Regex("^bytes=(\\d+)-(\\d+)$").find(value.orEmpty()) ?: return null
        val start = match.groupValues[1].toInt()
        val end = minOf(match.groupValues[2].toInt(), total - 1)
        return start to end
    }

    private fun sha256(content: ByteArray): String =
        MessageDigest.getInstance("SHA-256").digest(content)
            .joinToString("") { "%02x".format(it) }

    private fun sha256(file: File): String {
        val digest = MessageDigest.getInstance("SHA-256")
        file.inputStream().use { input ->
            val buffer = ByteArray(128 * 1024)
            while (true) {
                val read = input.read(buffer)
                if (read <= 0) break
                digest.update(buffer, 0, read)
            }
        }
        return digest.digest().joinToString("") { "%02x".format(it) }
    }

    private fun verifyProductionRanges(url: String, file: File, total: Long) {
        val sampleSize = 1024 * 1024
        val starts = listOf(0L, total / 2, total - sampleSize)
        RandomAccessFile(file, "r").use { local ->
            starts.forEach { start ->
                val connection = java.net.URL(url).openConnection() as java.net.HttpURLConnection
                connection.setRequestProperty(
                    "Range",
                    "bytes=$start-${start + sampleSize - 1}",
                )
                connection.setRequestProperty("Accept-Encoding", "identity")
                assertTrue(connection.responseCode == 206)
                val remote = connection.inputStream.use { it.readNBytes(sampleSize) }
                val expected = ByteArray(sampleSize)
                local.seek(start)
                local.readFully(expected)
                assertArrayEquals(expected, remote)
                connection.disconnect()
            }
        }
    }

    private class FileRangeServer(
        private val file: File,
    ) : AutoCloseable {
        private val running = AtomicBoolean(true)
        private val socket = ServerSocket(0, 50, InetAddress.getByName("127.0.0.1"))
        private val executor = Executors.newFixedThreadPool(12)
        val url = "http://127.0.0.1:${socket.localPort}/production.apk"

        init {
            Thread {
                while (running.get()) {
                    try {
                        val client = socket.accept()
                        executor.execute { handle(client) }
                    } catch (error: Exception) {
                        if (running.get()) throw error
                    }
                }
            }.start()
        }

        private fun handle(client: java.net.Socket) {
            client.use { connection ->
                val reader = connection.getInputStream().bufferedReader()
                var rangeHeader = ""
                while (true) {
                    val line = reader.readLine() ?: break
                    if (line.isEmpty()) break
                    if (line.startsWith("Range:", ignoreCase = true)) {
                        rangeHeader = line.substringAfter(':').trim()
                    }
                }
                val range = parseRange(rangeHeader, file.length().toInt())
                val output = connection.getOutputStream()
                if (range == null) {
                    output.write(
                        buildString {
                            append("HTTP/1.1 200 OK\r\n")
                            append("Content-Length: ${file.length()}\r\n")
                            append("Connection: close\r\n\r\n")
                        }.toByteArray(),
                    )
                    file.inputStream().use { it.copyTo(output) }
                    return@use
                }
                val length = range.second - range.first + 1
                output.write(
                    buildString {
                        append("HTTP/1.1 206 Partial Content\r\n")
                        append("Content-Length: $length\r\n")
                        append("Content-Range: bytes ${range.first}-${range.second}/${file.length()}\r\n")
                        append("Accept-Ranges: bytes\r\n")
                        append("ETag: \"production-artifact\"\r\n")
                        append("Connection: close\r\n\r\n")
                    }.toByteArray(),
                )
                RandomAccessFile(file, "r").use { input ->
                    input.seek(range.first.toLong())
                    val buffer = ByteArray(128 * 1024)
                    var remaining = length
                    while (remaining > 0) {
                        val read = input.read(buffer, 0, minOf(buffer.size, remaining))
                        if (read <= 0) break
                        output.write(buffer, 0, read)
                        remaining -= read
                    }
                }
                output.flush()
            }
        }

        private fun parseRange(value: String, total: Int): Pair<Int, Int>? {
            val match = Regex("^bytes=(\\d+)-(\\d+)$").find(value) ?: return null
            val start = match.groupValues[1].toInt()
            val end = minOf(match.groupValues[2].toInt(), total - 1)
            return start to end
        }

        override fun close() {
            running.set(false)
            socket.close()
            executor.shutdownNow()
        }
    }

    private class TestHttpServer(
        private val content: ByteArray,
        private val supportRanges: Boolean,
        private val delay: ((() -> Unit) -> Unit)?,
    ) : AutoCloseable {
        private val running = AtomicBoolean(true)
        private val socket = ServerSocket(0, 50, InetAddress.getByName("127.0.0.1"))
        private val executor = Executors.newFixedThreadPool(8)
        val url = "http://127.0.0.1:${socket.localPort}/update.apk"

        init {
            Thread {
                while (running.get()) {
                    try {
                        val client = socket.accept()
                        executor.execute { handle(client) }
                    } catch (error: Exception) {
                        if (running.get()) throw error
                    }
                }
            }.start()
        }

        private fun handle(client: java.net.Socket) {
            client.use { connection ->
                val reader = connection.getInputStream().bufferedReader()
                var rangeHeader = ""
                while (true) {
                    val line = reader.readLine() ?: break
                    if (line.isEmpty()) break
                    if (line.startsWith("Range:", ignoreCase = true)) {
                        rangeHeader = line.substringAfter(':').trim()
                    }
                }
                val range = parseRange(rangeHeader, content.size)
                val task = {
                    val output = connection.getOutputStream()
                    if (!supportRanges || range == null) {
                        writeResponse(output, 200, content, emptyMap())
                    } else {
                        val chunk = content.copyOfRange(range.first, range.second + 1)
                        writeResponse(
                            output,
                            206,
                            chunk,
                            mapOf(
                                "Content-Range" to
                                    "bytes ${range.first}-${range.second}/${content.size}",
                            ),
                        )
                    }
                }
                if (delay != null) delay(task) else task()
            }
        }

        private fun writeResponse(
            output: java.io.OutputStream,
            status: Int,
            body: ByteArray,
            extraHeaders: Map<String, String>,
        ) {
            val reason = if (status == 206) "Partial Content" else "OK"
            val headers = buildString {
                append("HTTP/1.1 $status $reason\r\n")
                append("Content-Type: application/vnd.android.package-archive\r\n")
                append("Content-Length: ${body.size}\r\n")
                append("Accept-Ranges: bytes\r\n")
                append("ETag: \"fixture-v1\"\r\n")
                append("Last-Modified: Thu, 23 Oct 2026 00:00:00 GMT\r\n")
                append("Connection: close\r\n")
                extraHeaders.forEach { (key, value) -> append("$key: $value\r\n") }
                append("\r\n")
            }
            output.write(headers.toByteArray())
            output.write(body)
            output.flush()
        }

        private fun parseRange(value: String, total: Int): Pair<Int, Int>? {
            val match = Regex("^bytes=(\\d+)-(\\d+)$").find(value) ?: return null
            val start = match.groupValues[1].toInt()
            val end = minOf(match.groupValues[2].toInt(), total - 1)
            return start to end
        }

        override fun close() {
            running.set(false)
            socket.close()
            executor.shutdownNow()
        }
    }
}
