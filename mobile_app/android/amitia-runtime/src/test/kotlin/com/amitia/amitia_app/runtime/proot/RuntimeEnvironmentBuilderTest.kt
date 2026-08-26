package com.amitia.amitia_app.runtime.proot

import com.amitia.amitia_app.runtime.connection.BackendEndpointPolicy
import com.amitia.amitia_app.runtime.install.RuntimeHostLayout
import com.amitia.amitia_app.runtime.install.internal.DefaultRuntimeHostLayout
import com.amitia.amitia_app.runtime.proot.internal.DefaultRuntimeEnvironmentBuilder
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

class RuntimeEnvironmentBuilderTest {

    @get:Rule
    val tempFolder = TemporaryFolder()

    private fun createLayout(): RuntimeHostLayout {
        val base = tempFolder.newFolder("proot-env")
        return DefaultRuntimeHostLayout(
            controlBaseDir = File(base, "control"),
            dataBaseDir = File(base, "data"),
        )
    }

    private fun createBuilder(): DefaultRuntimeEnvironmentBuilder {
        return DefaultRuntimeEnvironmentBuilder()
    }

    @Test
    fun buildReturnsSuccessWithValidLayout() {
        val layout = createLayout()
        val builder = createBuilder()
        val request = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = BackendEndpointPolicy("127.0.0.1", 18899, "http", "ws")
        )
        val result = builder.build(request)
        assertTrue(result is RuntimeEnvironmentResult.Success)
    }

    @Test
    fun guestRuntimeContainsAllRequiredRoots() {
        val layout = createLayout()
        val builder = createBuilder()
        val request = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = BackendEndpointPolicy("127.0.0.1", 18899, "http", "ws")
        )
        val result = builder.build(request) as RuntimeEnvironmentResult.Success
        val env = result.environment

        assertEquals("/opt/amitia", env.guestRuntime["AMITIA_RUNTIME_ROOT"])
        assertEquals("/etc/amitia", env.guestRuntime["AMITIA_CONFIG_ROOT"])
        assertEquals("/var/lib/amitia", env.guestRuntime["AMITIA_DATA_ROOT"])
        assertEquals("/var/cache/amitia", env.guestRuntime["AMITIA_CACHE_ROOT"])
        assertEquals("/var/log/amitia", env.guestRuntime["AMITIA_LOG_ROOT"])
        assertEquals("/run/amitia", env.guestRuntime["AMITIA_RUN_ROOT"])
        assertEquals("/run/amitia/tmp", env.guestRuntime["AMITIA_TEMP_ROOT"])
        assertEquals("/var/lib/amitia/workspaces", env.guestRuntime["AMITIA_WORKSPACE_ROOT"])
        assertEquals("/home/amitia", env.guestRuntime["AMITIA_HOME"])
    }

    @Test
    fun homeIsCorrect() {
        val layout = createLayout()
        val builder = createBuilder()
        val request = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = BackendEndpointPolicy("127.0.0.1", 18899, "http", "ws")
        )
        val result = builder.build(request) as RuntimeEnvironmentResult.Success
        assertEquals("/home/amitia", result.environment.guestRuntime["HOME"])
    }

    @Test
    fun localeIsCorrect() {
        val layout = createLayout()
        val builder = createBuilder()
        val request = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = BackendEndpointPolicy("127.0.0.1", 18899, "http", "ws")
        )
        val result = builder.build(request) as RuntimeEnvironmentResult.Success
        assertEquals("C.UTF-8", result.environment.guestRuntime["LANG"])
        assertEquals("C.UTF-8", result.environment.guestRuntime["LC_ALL"])
        assertEquals("Etc/UTC", result.environment.guestRuntime["TZ"])
    }

    @Test
    fun pathStartsWithNodeBin() {
        val layout = createLayout()
        val builder = createBuilder()
        val request = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = BackendEndpointPolicy("127.0.0.1", 18899, "http", "ws")
        )
        val result = builder.build(request) as RuntimeEnvironmentResult.Success
        val path = result.environment.guestRuntime["PATH"]
        assertNotNull(path)
        assertTrue(path!!.startsWith("/opt/amitia/node/bin"))
    }

    @Test
    fun pathDoesNotContainHostPaths() {
        val layout = createLayout()
        val builder = createBuilder()
        val request = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = BackendEndpointPolicy("127.0.0.1", 18899, "http", "ws")
        )
        val result = builder.build(request) as RuntimeEnvironmentResult.Success
        val path = result.environment.guestRuntime["PATH"]!!
        assertFalse(path.contains("/system/bin"))
        assertFalse(path.contains("/system/xbin"))
        assertFalse(path.contains("/data/local/tmp"))
    }

    @Test
    fun nodeOptionsNotInGuest() {
        val layout = createLayout()
        val builder = createBuilder()
        val request = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = BackendEndpointPolicy("127.0.0.1", 18899, "http", "ws")
        )
        val result = builder.build(request) as RuntimeEnvironmentResult.Success
        assertFalse(result.environment.guestRuntime.containsKey("NODE_OPTIONS"))
    }

    @Test
    fun nodePathNotInGuest() {
        val layout = createLayout()
        val builder = createBuilder()
        val request = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = BackendEndpointPolicy("127.0.0.1", 18899, "http", "ws")
        )
        val result = builder.build(request) as RuntimeEnvironmentResult.Success
        assertFalse(result.environment.guestRuntime.containsKey("NODE_PATH"))
    }

    @Test
    fun qdrantOverridesNotInGuest() {
        val layout = createLayout()
        val builder = createBuilder()
        val request = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = BackendEndpointPolicy("127.0.0.1", 18899, "http", "ws")
        )
        val result = builder.build(request) as RuntimeEnvironmentResult.Success
        for ((key, _) in result.environment.guestRuntime) {
            assertFalse("QDRANT__ prefix must not exist: $key", key.startsWith("QDRANT__"))
        }
    }

    @Test
    fun proxyNotInGuestRuntime() {
        val layout = createLayout()
        val builder = createBuilder()
        val request = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = BackendEndpointPolicy("127.0.0.1", 18899, "http", "ws")
        )
        val result = builder.build(request) as RuntimeEnvironmentResult.Success
        assertFalse(result.environment.guestRuntime.containsKey("HTTP_PROXY"))
        assertFalse(result.environment.guestRuntime.containsKey("HTTPS_PROXY"))
        assertFalse(result.environment.guestRuntime.containsKey("ALL_PROXY"))
        assertFalse(result.environment.guestRuntime.containsKey("http_proxy"))
        assertFalse(result.environment.guestRuntime.containsKey("https_proxy"))
        assertFalse(result.environment.guestRuntime.containsKey("all_proxy"))
    }

    @Test
    fun ldPreloadNotInGuestRuntime() {
        val layout = createLayout()
        val builder = createBuilder()
        val request = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = BackendEndpointPolicy("127.0.0.1", 18899, "http", "ws")
        )
        val result = builder.build(request) as RuntimeEnvironmentResult.Success
        assertFalse(result.environment.guestRuntime.containsKey("LD_PRELOAD"))
    }

    @Test
    fun javaHomeNotInGuestRuntime() {
        val layout = createLayout()
        val builder = createBuilder()
        val request = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = BackendEndpointPolicy("127.0.0.1", 18899, "http", "ws")
        )
        val result = builder.build(request) as RuntimeEnvironmentResult.Success
        assertFalse(result.environment.guestRuntime.containsKey("JAVA_HOME"))
        assertFalse(result.environment.guestRuntime.containsKey("ANDROID_HOME"))
    }

    @Test
    fun goEnvNotInGuestRuntime() {
        val layout = createLayout()
        val builder = createBuilder()
        val request = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = BackendEndpointPolicy("127.0.0.1", 18899, "http", "ws")
        )
        val result = builder.build(request) as RuntimeEnvironmentResult.Success
        assertFalse(result.environment.guestRuntime.containsKey("GOROOT"))
        assertFalse(result.environment.guestRuntime.containsKey("GOPATH"))
        assertFalse(result.environment.guestRuntime.containsKey("GOMODCACHE"))
    }

    @Test
    fun secretsNotInGuestRuntime() {
        val layout = createLayout()
        val builder = createBuilder()
        val request = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = BackendEndpointPolicy("127.0.0.1", 18899, "http", "ws")
        )
        val result = builder.build(request) as RuntimeEnvironmentResult.Success
        assertFalse(result.environment.guestRuntime.containsKey("OPENAI_API_KEY"))
        assertFalse(result.environment.guestRuntime.containsKey("ANTHROPIC_API_KEY"))
        assertFalse(result.environment.guestRuntime.containsKey("DEEPSEEK_API_KEY"))
        assertFalse(result.environment.guestRuntime.containsKey("GITHUB_TOKEN"))
        assertFalse(result.environment.guestRuntime.containsKey("NPM_TOKEN"))
    }

    @Test
    fun localTokenNotInEnvironment() {
        val layout = createLayout()
        val builder = createBuilder()
        val request = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = BackendEndpointPolicy("127.0.0.1", 18899, "http", "ws")
        )
        val result = builder.build(request) as RuntimeEnvironmentResult.Success
        assertFalse(result.environment.guestRuntime.containsKey("AMITIA_LOCAL_TOKEN"))
    }

    @Test
    fun hostPathsNotInGuestValues() {
        val layout = createLayout()
        val builder = createBuilder()
        val request = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = BackendEndpointPolicy("127.0.0.1", 18899, "http", "ws")
        )
        val result = builder.build(request) as RuntimeEnvironmentResult.Success
        for ((key, value) in result.environment.guestRuntime) {
            assertFalse("Guest env value for '$key' must not contain /data/user/", value.contains("/data/user/"))
            assertFalse("Guest env value for '$key' must not contain /data/data/", value.contains("/data/data/"))
            assertFalse("Guest env value for '$key' must not contain /sdcard/", value.contains("/sdcard/"))
            assertFalse("Guest env value for '$key' must not contain /storage/emulated/", value.contains("/storage/emulated/"))
        }
    }

    @Test
    fun backendEndpointIsCorrect() {
        val layout = createLayout()
        val builder = createBuilder()
        val request = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = BackendEndpointPolicy("127.0.0.1", 18899, "http", "ws")
        )
        val result = builder.build(request) as RuntimeEnvironmentResult.Success
        assertEquals("127.0.0.1", result.environment.guestRuntime["AMITIA_SERVER_HOST"])
        assertEquals("18899", result.environment.guestRuntime["AMITIA_SERVER_PORT"])
    }

    @Test
    fun invalidEndpointHostReturnsFailure() {
        val layout = createLayout()
        val builder = createBuilder()
        val request = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = BackendEndpointPolicy("0.0.0.0", 18899, "http", "ws")
        )
        val result = builder.build(request)
        assertTrue(result is RuntimeEnvironmentResult.Failure)
        assertEquals(RuntimeEnvironmentErrorCode.ENDPOINT_INVALID, (result as RuntimeEnvironmentResult.Failure).code)
    }

    @Test
    fun invalidEndpointPortReturnsFailure() {
        val layout = createLayout()
        val builder = createBuilder()
        val request = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = BackendEndpointPolicy("127.0.0.1", 3000, "http", "ws")
        )
        val result = builder.build(request)
        assertTrue(result is RuntimeEnvironmentResult.Failure)
        assertEquals(RuntimeEnvironmentErrorCode.ENDPOINT_INVALID, (result as RuntimeEnvironmentResult.Failure).code)
    }

    @Test
    fun environmentIsDeterministic() {
        val layout = createLayout()
        val builder = createBuilder()
        val request = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = BackendEndpointPolicy("127.0.0.1", 18899, "http", "ws")
        )
        val result1 = builder.build(request) as RuntimeEnvironmentResult.Success
        val result2 = builder.build(request) as RuntimeEnvironmentResult.Success
        assertEquals(result1.environment.guestRuntime, result2.environment.guestRuntime)
        assertEquals(result1.environment.hostProcess, result2.environment.hostProcess)
    }

    @Test
    fun tempRootIsGuestTmp() {
        val layout = createLayout()
        val builder = createBuilder()
        val request = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = BackendEndpointPolicy("127.0.0.1", 18899, "http", "ws")
        )
        val result = builder.build(request) as RuntimeEnvironmentResult.Success
        val tempRoot = result.environment.guestRuntime["AMITIA_TEMP_ROOT"]
        assertEquals("/run/amitia/tmp", tempRoot)
    }

    @Test
    fun workspaceRootIsWithinDataRoot() {
        val layout = createLayout()
        val builder = createBuilder()
        val request = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = BackendEndpointPolicy("127.0.0.1", 18899, "http", "ws")
        )
        val result = builder.build(request) as RuntimeEnvironmentResult.Success
        val workspaceRoot = result.environment.guestRuntime["AMITIA_WORKSPACE_ROOT"]
        val dataRoot = result.environment.guestRuntime["AMITIA_DATA_ROOT"]
        assertTrue(workspaceRoot!!.startsWith(dataRoot!!))
    }

    @Test
    fun hostProcessHasTmpdir() {
        val layout = createLayout()
        val builder = createBuilder()
        val request = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = BackendEndpointPolicy("127.0.0.1", 18899, "http", "ws")
        )
        val result = builder.build(request) as RuntimeEnvironmentResult.Success
        assertNotNull(result.environment.hostProcess["TMPDIR"])
        assertFalse(result.environment.hostProcess["TMPDIR"]!!.contains("/run/amitia"))
    }

    @Test
    fun noProxyIsSet() {
        val layout = createLayout()
        val builder = createBuilder()
        val request = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = BackendEndpointPolicy("127.0.0.1", 18899, "http", "ws")
        )
        val result = builder.build(request) as RuntimeEnvironmentResult.Success
        assertEquals("127.0.0.1,localhost", result.environment.guestRuntime["NO_PROXY"])
        assertEquals("127.0.0.1,localhost", result.environment.guestRuntime["no_proxy"])
    }
    @Test
    fun androidRuntimeModeAndDirectoryAliasesAreExplicit() {
        val layout = createLayout()
        val builder = createBuilder()
        val request = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = BackendEndpointPolicy("127.0.0.1", 18899, "http", "ws")
        )
        val result = builder.build(request) as RuntimeEnvironmentResult.Success
        val guest = result.environment.guestRuntime

        assertEquals("android-proot", guest["AMITIA_RUNTIME_MODE"])
        assertEquals("local_single_user", guest["AMITIA_SECURITY_MODE"])
        assertEquals("false", guest["AMITIA_ALLOW_REMOTE_ACCESS"])
        assertEquals("/etc/amitia", guest["AMITIA_CONFIG_DIR"])
        assertEquals("/var/lib/amitia", guest["AMITIA_DATA_DIR"])
        assertEquals("/var/cache/amitia", guest["AMITIA_CACHE_DIR"])
        assertEquals("/var/log/amitia", guest["AMITIA_LOG_DIR"])
        assertEquals("/run/amitia/tmp", guest["AMITIA_TEMP_DIR"])
        assertEquals("/var/lib/amitia/workspaces", guest["AMITIA_WORKSPACE_DIR"])
        assertEquals("/var/lib/amitia/security/local-token", guest["AMITIA_LOCAL_TOKEN_FILE"])
        assertEquals("false", guest["AMITIA_GRAPH_STORE_ENABLED"])
        assertEquals("false", guest["AMITIA_GRAPH_STORE_REQUIRED"])
    }

    @Test
    fun hostProcessContainsDedicatedProotTempDirectory() {
        val layout = createLayout()
        val builder = createBuilder()
        val request = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = BackendEndpointPolicy("127.0.0.1", 18899, "http", "ws")
        )
        val result = builder.build(request) as RuntimeEnvironmentResult.Success
        val prootTmp = result.environment.hostProcess["PROOT_TMP_DIR"]
        assertNotNull(prootTmp)
        assertTrue(prootTmp!!.endsWith("/run/proot-tmp"))
        assertFalse(prootTmp.contains("/run/amitia"))
    }

}
