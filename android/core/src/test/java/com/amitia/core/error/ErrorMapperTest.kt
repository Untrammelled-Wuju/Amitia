package com.amitia.core.error

import com.amitia.core.network.client.AmitiaApiException
import com.google.common.truth.Truth.assertThat
import org.junit.Test
import java.io.IOException
import java.net.SocketTimeoutException
import java.net.UnknownHostException

class ErrorMapperTest {

    private val mapper = ErrorMapper()

    @Test
    fun map_passes_AmitiaError_through() {
        val original = AmitiaError.RootfsNotInstalled.create()

        val mapped = mapper.map(original)

        assertThat(mapped).isSameInstanceAs(original)
    }

    @Test
    fun map_AmitiaApiException_NetworkUnavailable_to_AmitiaError() {
        val ex = AmitiaApiException.NetworkUnavailable

        val mapped = mapper.map(ex)

        assertThat(mapped).isInstanceOf(AmitiaError.NetworkUnavailable::class.java)
        assertThat(mapped.code).isEqualTo(AmitiaError.CODE_NETWORK_UNAVAILABLE)
    }

    @Test
    fun map_AmitiaApiException_RemoteUnreachable_carries_url() {
        val ex = AmitiaApiException.RemoteUnreachable("https://api.example.com")

        val mapped = mapper.map(ex)

        assertThat(mapped).isInstanceOf(AmitiaError.RemoteUnreachable::class.java)
        assertThat((mapped as AmitiaError.RemoteUnreachable).url).isEqualTo("https://api.example.com")
    }

    @Test
    fun map_AmitiaApiException_TokenExpired_to_AmitiaError() {
        val mapped = mapper.map(AmitiaApiException.TokenExpired)

        assertThat(mapped).isInstanceOf(AmitiaError.TokenExpired::class.java)
        assertThat(mapped.code).isEqualTo(AmitiaError.CODE_TOKEN_EXPIRED)
    }

    @Test
    fun map_AmitiaApiException_Timeout_to_ServiceTimeout() {
        val mapped = mapper.map(AmitiaApiException.Timeout)

        assertThat(mapped).isInstanceOf(AmitiaError.ServiceTimeout::class.java)
        assertThat((mapped as AmitiaError.ServiceTimeout).service).isEqualTo("backend")
    }

    @Test
    fun map_AmitiaApiException_SseDisconnected_to_StreamDisconnected() {
        val mapped = mapper.map(AmitiaApiException.SseDisconnected("eof"))

        assertThat(mapped).isInstanceOf(AmitiaError.StreamDisconnected::class.java)
    }

    @Test
    fun map_AmitiaApiException_UploadFailed_to_UploadFailed() {
        val mapped = mapper.map(AmitiaApiException.UploadFailed("too large"))

        assertThat(mapped).isInstanceOf(AmitiaError.UploadFailed::class.java)
        assertThat((mapped as AmitiaError.UploadFailed).message).contains("too large")
    }

    @Test
    fun map_AmitiaApiException_AudioFailed_to_AudioFailed() {
        val mapped = mapper.map(AmitiaApiException.AudioFailed("decode error"))

        assertThat(mapped).isInstanceOf(AmitiaError.AudioFailed::class.java)
    }

    @Test
    fun map_AmitiaApiException_MigrationFailed_to_MigrationFailed() {
        val mapped = mapper.map(AmitiaApiException.MigrationFailed("schema v2"))

        assertThat(mapped).isInstanceOf(AmitiaError.MigrationFailed::class.java)
    }

    @Test
    fun map_AmitiaApiException_ServerError_to_Unknown() {
        val mapped = mapper.map(AmitiaApiException.ServerError(500, "oops"))

        assertThat(mapped).isInstanceOf(AmitiaError.Unknown::class.java)
        assertThat(mapped.message).contains("HTTP 500")
    }

    @Test
    fun map_AmitiaApiException_NotFound_to_Unknown_with_path() {
        val mapped = mapper.map(AmitiaApiException.NotFound("/api/missing"))

        assertThat(mapped).isInstanceOf(AmitiaError.Unknown::class.java)
        assertThat(mapped.message).contains("/api/missing")
    }

    @Test
    fun map_AmitiaApiException_Unwraps_raw_throwable() {
        val inner = IOException("inner boom")
        val wrapped = AmitiaApiException.Unknown(inner)

        val mapped = mapper.map(wrapped)

        assertThat(mapped).isInstanceOf(AmitiaError.NetworkUnavailable::class.java)
    }

    @Test
    fun map_SocketTimeoutException_to_ServiceTimeout() {
        val mapped = mapper.map(SocketTimeoutException("read timed out"))

        assertThat(mapped).isInstanceOf(AmitiaError.ServiceTimeout::class.java)
    }

    @Test
    fun map_UnknownHostException_to_NetworkUnavailable() {
        val mapped = mapper.map(UnknownHostException("api.example.com"))

        assertThat(mapped).isInstanceOf(AmitiaError.NetworkUnavailable::class.java)
    }

    @Test
    fun map_IOException_to_NetworkUnavailable() {
        val mapped = mapper.map(IOException("broken pipe"))

        assertThat(mapped).isInstanceOf(AmitiaError.NetworkUnavailable::class.java)
    }

    @Test
    fun map_generic_throwable_to_Unknown() {
        val mapped = mapper.map(IllegalStateException("generic"))

        assertThat(mapped).isInstanceOf(AmitiaError.Unknown::class.java)
    }

    @Test
    fun mapFailedState_rootfs_message_to_RootfsInstallFailed() {
        val mapped = mapper.mapFailedState("RootFS install failed", retryable = true, requiresUserAction = true)

        assertThat(mapped).isInstanceOf(AmitiaError.RootfsInstallFailed::class.java)
    }

    @Test
    fun mapFailedState_qdrant_message_to_QdrantStartFailed() {
        val mapped = mapper.mapFailedState("qdrant startup error", retryable = true, requiresUserAction = false)

        assertThat(mapped).isInstanceOf(AmitiaError.QdrantStartFailed::class.java)
    }

    @Test
    fun mapFailedState_surreal_message_to_SurrealdbStartFailed() {
        val mapped = mapper.mapFailedState("SurrealDB crashed", retryable = true, requiresUserAction = false)

        assertThat(mapped).isInstanceOf(AmitiaError.SurrealdbStartFailed::class.java)
    }

    @Test
    fun mapFailedState_backend_message_to_BackendStartFailed() {
        val mapped = mapper.mapFailedState("backend port in use", retryable = true, requiresUserAction = false)

        assertThat(mapped).isInstanceOf(AmitiaError.BackendStartFailed::class.java)
    }

    @Test
    fun mapFailedState_port_message_to_PortConflict_with_extracted_port() {
        val mapped = mapper.mapFailedState("address already in use 18899", retryable = true, requiresUserAction = true)

        assertThat(mapped).isInstanceOf(AmitiaError.PortConflict::class.java)
        assertThat((mapped as AmitiaError.PortConflict).port).isEqualTo(18899)
    }

    @Test
    fun mapFailedState_timeout_message_to_ServiceTimeout() {
        val mapped = mapper.mapFailedState("operation timeout", retryable = true, requiresUserAction = false)

        assertThat(mapped).isInstanceOf(AmitiaError.ServiceTimeout::class.java)
        assertThat((mapped as AmitiaError.ServiceTimeout).service).isEqualTo("runtime")
    }

    @Test
    fun mapFailedState_permission_message_to_DataDirNoPermission() {
        val mapped = mapper.mapFailedState("permission denied", retryable = false, requiresUserAction = true)

        assertThat(mapped).isInstanceOf(AmitiaError.DataDirNoPermission::class.java)
    }

    @Test
    fun mapFailedState_killed_message_to_RuntimeKilled() {
        val mapped = mapper.mapFailedState("process killed by system", retryable = true, requiresUserAction = false)

        assertThat(mapped).isInstanceOf(AmitiaError.RuntimeKilled::class.java)
    }

    @Test
    fun mapFailedState_binary_message_to_BinaryIncompatible() {
        val mapped = mapper.mapFailedState("binary incompatible with arm64", retryable = false, requiresUserAction = true)

        assertThat(mapped).isInstanceOf(AmitiaError.BinaryIncompatible::class.java)
    }

    @Test
    fun toUserMessage_returns_localized_message_for_RootfsNotInstalled() {
        val message = mapper.toUserMessage(AmitiaError.RootfsNotInstalled.create())

        assertThat(message).contains("根文件系统")
    }

    @Test
    fun toUserMessage_returns_message_for_TokenExpired() {
        val message = mapper.toUserMessage(AmitiaError.TokenExpired.create())

        assertThat(message).contains("登录已过期")
    }

    @Test
    fun toUserMessage_returns_message_for_PortConflict() {
        val message = mapper.toUserMessage(AmitiaError.PortConflict.create(18899))

        assertThat(message).contains("18899")
    }

    @Test
    fun toDiagnosticInfo_includes_error_code_and_retryable_flag() {
        val info = mapper.toDiagnosticInfo(AmitiaError.QdrantStartFailed.create("boom"))

        assertThat(info).contains("Error code: QDRANT_START_FAILED")
        assertThat(info).contains("Retryable: true")
    }
}
