package com.amitia.core.network.client

import javax.inject.Inject
import javax.inject.Singleton
import okhttp3.Interceptor
import okhttp3.Response

@Singleton
class LoggingInterceptor @Inject constructor() : Interceptor {

    override fun intercept(chain: Interceptor.Chain): Response {
        val request = chain.request()
        val startedAt = System.currentTimeMillis()
        val sanitizedUrl = sanitizeUrl(request.url.toString())
        val sanitizedHeaders = sanitizeHeaders(request.headers.toString())

        android.util.Log.d(TAG, "--> ${request.method} $sanitizedUrl")
        if (sanitizedHeaders.isNotBlank()) {
            android.util.Log.d(TAG, "Headers: $sanitizedHeaders")
        }

        val response = chain.proceed(request)
        val durationMs = System.currentTimeMillis() - startedAt

        android.util.Log.d(
            TAG,
            "<-- ${response.code} $sanitizedUrl (${durationMs}ms, ${response.body?.contentLength() ?: -1} bytes)"
        )

        val peeked = response.peekBody(MAX_PEEK_BYTES)
        val bodyText = peeked.string()
        val sanitizedBody = sanitizeBody(bodyText)
        if (sanitizedBody.isNotBlank()) {
            android.util.Log.d(TAG, "Body: $sanitizedBody")
        }

        return response
    }

    private fun sanitizeUrl(url: String): String {
        return url.replace(Regex("(token|access_token|password)=[^&]+"), "$1=***")
    }

    private fun sanitizeHeaders(headers: String): String {
        var result = headers
        result = result.replace(
            Regex("(?i)(Authorization|Token|X-Api-Key|Cookie):\\s*[^\\r\\n]+"),
            "$1: ***"
        )
        return result
    }

    private fun sanitizeBody(body: String): String {
        if (body.length > MAX_LOG_BODY_LENGTH) {
            return body.take(MAX_LOG_BODY_LENGTH) + "...[truncated]"
        }
        var result = body
        result = result.replace(
            Regex("(?i)\"(token|password|secret|access_token|refresh_token|authorization|content)\"\\s*:\\s*\"[^\"]*\""),
            "\"$1\":\"***\""
        )
        return result
    }

    companion object {
        private const val TAG = "AmitiaHttp"
        private const val MAX_PEEK_BYTES = 1024L * 16L
        private const val MAX_LOG_BODY_LENGTH = 2048
    }
}
