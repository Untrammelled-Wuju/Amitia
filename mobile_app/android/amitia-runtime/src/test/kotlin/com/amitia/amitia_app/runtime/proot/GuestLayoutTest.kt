package com.amitia.amitia_app.runtime.proot

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class GuestLayoutTest {

    @Test
    fun programRoot_isOptAmitia() {
        assertEquals("/opt/amitia", GuestLayout.PROGRAM)
    }

    @Test
    fun configRoot_isEtcAmitia() {
        assertEquals("/etc/amitia", GuestLayout.CONFIG)
    }

    @Test
    fun dataRoot_isVarLibAmitia() {
        assertEquals("/var/lib/amitia", GuestLayout.DATA)
    }

    @Test
    fun cacheRoot_isVarCacheAmitia() {
        assertEquals("/var/cache/amitia", GuestLayout.CACHE)
    }

    @Test
    fun logsRoot_isVarLogAmitia() {
        assertEquals("/var/log/amitia", GuestLayout.LOGS)
    }

    @Test
    fun runRoot_isRunAmitia() {
        assertEquals("/run/amitia", GuestLayout.RUN)
    }

    @Test
    fun homeRoot_isHomeAmitia() {
        assertEquals("/home/amitia", GuestLayout.HOME)
    }

    @Test
    fun tmpRoot_isRuntimeTmp() {
        assertEquals("/run/amitia/tmp", GuestLayout.TMP)
    }

    @Test
    fun tmpIsWithinWritableRunRoot() {
        assertTrue("GuestLayout.TMP must be within RUN", GuestLayout.TMP.startsWith(GuestLayout.RUN + "/"))
    }

    @Test
    fun backendServerPath_isDerivedFromProgram() {
        assertEquals("/opt/amitia/backend/amitia-server", GuestLayout.BACKEND_SERVER)
        assertTrue(GuestLayout.BACKEND_SERVER.startsWith(GuestLayout.PROGRAM))
    }

    @Test
    fun nodeBinPath_isDerivedFromProgram() {
        assertEquals("/opt/amitia/node/bin", GuestLayout.NODE_BIN)
        assertTrue(GuestLayout.NODE_BIN.startsWith(GuestLayout.PROGRAM))
    }

    @Test
    fun qdrantBinPath_isDerivedFromProgram() {
        assertEquals("/opt/amitia/qdrant/bin/qdrant", GuestLayout.QDRANT_BIN)
        assertTrue(GuestLayout.QDRANT_BIN.startsWith(GuestLayout.PROGRAM))
    }

    @Test
    fun sidecarPaths_areDerivedFromProgram() {
        assertTrue(GuestLayout.SIDECAR_LAUNCHER.startsWith(GuestLayout.PROGRAM))
        assertTrue(GuestLayout.SIDECAR_BUNDLE.startsWith(GuestLayout.PROGRAM))
        assertTrue(GuestLayout.QQ_SIDECAR_LAUNCHER.startsWith(GuestLayout.PROGRAM))
        assertTrue(GuestLayout.QQ_SIDECAR_BUNDLE.startsWith(GuestLayout.PROGRAM))
    }

    @Test
    fun localTokenPath_isWithinDataRoot() {
        assertEquals("/var/lib/amitia/security/local-token", GuestLayout.LOCAL_TOKEN)
        assertTrue(GuestLayout.LOCAL_TOKEN.startsWith(GuestLayout.DATA))
    }

    @Test
    fun qdrantStorage_isWithinDataRoot() {
        assertTrue(GuestLayout.QDRANT_STORAGE.startsWith(GuestLayout.DATA))
    }

    @Test
    fun npmCache_isWithinCacheRoot() {
        assertEquals("/var/cache/amitia/npm", GuestLayout.NPM_CACHE)
        assertTrue(GuestLayout.NPM_CACHE.startsWith(GuestLayout.CACHE))
    }

    @Test
    fun path_startsWithNodeBin() {
        assertTrue(GuestLayout.PATH.startsWith(GuestLayout.NODE_BIN))
    }

    @Test
    fun path_doesNotContainHostPaths() {
        assertFalse(GuestLayout.PATH.contains("/system/bin"))
        assertFalse(GuestLayout.PATH.contains("/data"))
    }

    @Test
    fun allRoots_containsEightDistinctRoots() {
        assertEquals(8, GuestLayout.ALL_ROOTS.size)
        assertEquals(8, GuestLayout.ALL_ROOTS.toSet().size)
    }

    @Test
    fun allGuestPaths_areAbsolute() {
        val paths = listOf(
            GuestLayout.PROGRAM, GuestLayout.CONFIG, GuestLayout.DATA,
            GuestLayout.CACHE, GuestLayout.LOGS, GuestLayout.RUN,
            GuestLayout.HOME, GuestLayout.TMP,
            GuestLayout.BACKEND_SERVER, GuestLayout.NODE_BIN,
            GuestLayout.QDRANT_BIN, GuestLayout.SIDECAR_LAUNCHER,
            GuestLayout.LOCAL_TOKEN, GuestLayout.NPM_CACHE,
        )
        for (p in paths) {
            assertTrue("Path must be absolute: $p", p.startsWith("/"))
        }
    }

    @Test
    fun allGuestPaths_noTraversal() {
        val paths = listOf(
            GuestLayout.PROGRAM, GuestLayout.CONFIG, GuestLayout.DATA,
            GuestLayout.CACHE, GuestLayout.LOGS, GuestLayout.RUN,
            GuestLayout.HOME, GuestLayout.TMP,
            GuestLayout.BACKEND_SERVER, GuestLayout.NODE_BIN,
            GuestLayout.LOCAL_TOKEN,
        )
        for (p in paths) {
            assertFalse("Path must not contain ..: $p", p.contains(".."))
        }
    }

    @Test
    fun home_doesNotFallBackToRoot() {
        assertFalse(GuestLayout.HOME == "/root")
        assertFalse(HOME == "/home/ubuntu")
    }

    @Test
    fun programSubdirs_containsRequiredComponents() {
        assertTrue(GuestLayout.PROGRAM_SUBDIRS.contains("backend"))
        assertTrue(GuestLayout.PROGRAM_SUBDIRS.contains("node"))
        assertTrue(GuestLayout.PROGRAM_SUBDIRS.contains("qdrant"))
        assertTrue(GuestLayout.PROGRAM_SUBDIRS.contains("sidecar"))
        assertTrue(GuestLayout.PROGRAM_SUBDIRS.contains("qq-sidecar"))
    }

    private companion object {
        private const val HOME = GuestLayout.HOME
    }
}
