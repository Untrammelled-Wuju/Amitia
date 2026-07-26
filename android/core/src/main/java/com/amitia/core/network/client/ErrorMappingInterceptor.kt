package com.amitia.core.network.client

import java.io.IOException
import java.net.SocketTimeoutException
import java.net.UnknownHostException
import javax.inject.Inject
import javax.inject.Singleton
import okhttp3.Interceptor
import okhttp3.Response

@Singleton
class ErrorMappingInterceptor @Inject constructor() : Interceptor {

    override fun intercept(chain: Interceptor.Chain): Response {
        return try {
            val response = chain.proceed(chain.request())
            if (!response.isSuccessful) {
                throw mapHttpError(response, chain.request().url.encodedPath)
            }
            response
        } catch (e: SocketTimeoutException) {
            throw AmitiaApiException.Timeout
        } catch (e: UnknownHostException) {
            throw AmitiaApiException.RemoteUnreachable(chain.request().url.toString())
        } catch (e: javax.net.ssl.SSLException) {
            throw AmitiaApiException.RemoteUnreachable(chain.request().url.toString())
        } catch (e: java.net.ConnectException) {
            throw AmitiaApiException.RemoteUnreachable(chain.request().url.toString())
        } catch (e: AmitiaApiException) {
            throw e
        } catch (e: IOException) {
            throw AmitiaApiException.NetworkUnavailable
        }
    }

    private fun mapHttpError(response: Response, path: String): AmitiaApiException {
        val body = runCatching {
            response.peekBody(2048L).string()
        }.getOrNull()
        return when (response.code) {
            401 -> AmitiaApiException.TokenExpired
            403 -> AmitiaApiException.TokenExpired
            404 -> AmitiaApiException.NotFound(path)
            in 500..599 -> AmitiaApiException.ServerError(response.code, body)
            else -> AmitiaApiException.ServerError(response.code, body)
        }
    }
}
