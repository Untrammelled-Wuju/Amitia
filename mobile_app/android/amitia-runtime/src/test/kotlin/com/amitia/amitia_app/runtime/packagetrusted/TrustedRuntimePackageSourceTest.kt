package com.amitia.amitia_app.runtime.packagetrusted

import org.junit.Assert.assertEquals
import org.junit.Test
import java.io.File

class TrustedRuntimePackageSourceTest {

    @Test
    fun createReference_returnsStep7FrozenIdentity() {
        val packageFile = File("/data/local/tmp/packages/amitia-runtime-1.0.0.zip")
        val ref = TrustedRuntimePackageSource.createReference(packageFile)

        assertEquals("1.0.0", ref.expectedRuntimeVersion)
        assertEquals("3f061598a5c0b815cdb1d536694d9e251652be13f301fb215f1d1aae0c5f7f57", ref.expectedPackageSha256)
        assertEquals("linux", ref.expectedGuestOS)
        assertEquals("arm64", ref.expectedArchitecture)
        assertEquals(packageFile, ref.packageFile)
    }

    @Test
    fun expectedRuntimeVersion_returnsStep7Version() {
        assertEquals("1.0.0", TrustedRuntimePackageSource.expectedRuntimeVersion())
    }

    @Test
    fun expectedPackageSha256_returnsStep7Sha() {
        assertEquals(
            "3f061598a5c0b815cdb1d536694d9e251652be13f301fb215f1d1aae0c5f7f57",
            TrustedRuntimePackageSource.expectedPackageSha256()
        )
    }

    @Test
    fun expectedGuestOS_returnsLinux() {
        assertEquals("linux", TrustedRuntimePackageSource.expectedGuestOS())
    }

    @Test
    fun expectedArchitecture_returnsArm64() {
        assertEquals("arm64", TrustedRuntimePackageSource.expectedArchitecture())
    }

    @Test
    fun constants_matchStep7BuildRecord() {
        assertEquals("1.0.0", TrustedRuntimePackageSource.RUNTIME_VERSION)
        assertEquals(
            "3f061598a5c0b815cdb1d536694d9e251652be13f301fb215f1d1aae0c5f7f57",
            TrustedRuntimePackageSource.PACKAGE_SHA256
        )
        assertEquals("linux", TrustedRuntimePackageSource.GUEST_OS)
        assertEquals("arm64", TrustedRuntimePackageSource.ARCHITECTURE)
    }

    @Test
    fun createReference_withDifferentFile_returnsSameExpectedIdentity() {
        val packageFile1 = File("/tmp/test1.zip")
        val packageFile2 = File("/tmp/test2.zip")

        val ref1 = TrustedRuntimePackageSource.createReference(packageFile1)
        val ref2 = TrustedRuntimePackageSource.createReference(packageFile2)

        assertEquals(ref1.expectedRuntimeVersion, ref2.expectedRuntimeVersion)
        assertEquals(ref1.expectedPackageSha256, ref2.expectedPackageSha256)
        assertEquals(ref1.expectedGuestOS, ref2.expectedGuestOS)
        assertEquals(ref1.expectedArchitecture, ref2.expectedArchitecture)
    }
}
