package com.amitia.amitia_app.runtime.proot

import com.amitia.amitia_app.runtime.install.RuntimeHostLayout
import com.amitia.amitia_app.runtime.install.internal.DefaultRuntimeHostLayout
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

class MountContractTest {

    @get:Rule
    val tempFolder = TemporaryFolder()

    private fun createLayout(): DefaultRuntimeHostLayout {
        val controlBase = tempFolder.newFolder("noBackup")
        val dataBase = tempFolder.newFolder("data")
        return DefaultRuntimeHostLayout(controlBase, dataBase)
    }

    private fun createProgramSource(layout: DefaultRuntimeHostLayout, version: String = "1.0.0"): File {
        val dir = File(layout.runtimeVersionRoot(version), "program")
        dir.mkdirs()
        return dir
    }

    private fun unixPath(file: File): String = file.absolutePath.replace('\\', '/')

    @Test
    fun build_returnsAllSevenRoles() {
        val layout = createLayout()
        val source = createProgramSource(layout)
        val contract = MountContract.build(layout, source)
        assertEquals(7, contract.mounts.size)
        assertEquals(MountRole.PROGRAM, contract.mounts[0].role)
        assertEquals(MountRole.CONFIG, contract.mounts[1].role)
        assertEquals(MountRole.DATA, contract.mounts[2].role)
        assertEquals(MountRole.CACHE, contract.mounts[3].role)
        assertEquals(MountRole.LOGS, contract.mounts[4].role)
        assertEquals(MountRole.RUN, contract.mounts[5].role)
        assertEquals(MountRole.HOME, contract.mounts[6].role)
    }

    @Test
    fun build_programMountTargetsOptAmitia() {
        val layout = createLayout()
        val source = createProgramSource(layout)
        val contract = MountContract.build(layout, source)
        assertEquals(GuestLayout.PROGRAM, contract.programMount.guestTarget)
    }

    @Test
    fun build_configMountTargetsEtcAmitia() {
        val layout = createLayout()
        val source = createProgramSource(layout)
        val contract = MountContract.build(layout, source)
        assertEquals(GuestLayout.CONFIG, contract.configMount.guestTarget)
    }

    @Test
    fun build_dataMountTargetsVarLibAmitia() {
        val layout = createLayout()
        val source = createProgramSource(layout)
        val contract = MountContract.build(layout, source)
        assertEquals(GuestLayout.DATA, contract.dataMount.guestTarget)
    }

    @Test
    fun build_programMountIsReadOnly() {
        val layout = createLayout()
        val source = createProgramSource(layout)
        val contract = MountContract.build(layout, source)
        assertFalse(contract.programMount.writable)
    }

    @Test
    fun build_persistentMountsAreWritable() {
        val layout = createLayout()
        val source = createProgramSource(layout)
        val contract = MountContract.build(layout, source)
        assertTrue(contract.configMount.writable)
        assertTrue(contract.dataMount.writable)
        assertTrue(contract.cacheMount.writable)
        assertTrue(contract.logsMount.writable)
        assertTrue(contract.runMount.writable)
        assertTrue(contract.homeMount.writable)
    }

    @Test
    fun build_programSourceIsVersionChild() {
        val layout = createLayout()
        val source = createProgramSource(layout, "2.0.0")
        val contract = MountContract.build(layout, source)
        assertTrue(unixPath(File(contract.programMount.hostSource)).contains("versions/2.0.0"))
        assertEquals(source.absolutePath, contract.programMount.hostSource)
    }

    @Test
    fun build_configHostSourceIsPersistentConfigRoot() {
        val layout = createLayout()
        val source = createProgramSource(layout)
        val contract = MountContract.build(layout, source)
        assertEquals(layout.configRoot.absolutePath, contract.configMount.hostSource)
    }

    @Test
    fun build_dataHostSourceIsPersistentDataRoot() {
        val layout = createLayout()
        val source = createProgramSource(layout)
        val contract = MountContract.build(layout, source)
        assertEquals(layout.dataRoot.absolutePath, contract.dataMount.hostSource)
    }

    @Test(expected = IllegalArgumentException::class)
    fun build_rejectsVersionsRootAsProgramSource() {
        val layout = createLayout()
        MountContract.build(layout, layout.versionsRoot)
    }

    @Test(expected = IllegalArgumentException::class)
    fun build_rejectsNonExistentProgramSource() {
        val layout = createLayout()
        val nonExistent = File(layout.versionsRoot, "nonexistent/program")
        MountContract.build(layout, nonExistent)
    }

    @Test(expected = IllegalArgumentException::class)
    fun build_rejectsFileAsProgramSource() {
        val layout = createLayout()
        val file = File(layout.runtimeVersionRoot("1.0.0"), "file.txt")
        file.parentFile?.mkdirs()
        file.createNewFile()
        MountContract.build(layout, file)
    }

    @Test
    fun build_guestTargetsAreUnique() {
        val layout = createLayout()
        val source = createProgramSource(layout)
        val contract = MountContract.build(layout, source)
        val targets = contract.guestTargets
        assertEquals(targets.size, targets.toSet().size)
    }

    @Test
    fun build_guestTargetsAreAllAbsolute() {
        val layout = createLayout()
        val source = createProgramSource(layout)
        val contract = MountContract.build(layout, source)
        for (target in contract.guestTargets) {
            assertTrue("Guest target must be absolute: $target", target.startsWith("/"))
        }
    }

    @Test
    fun build_guestTargetsNoneIsRoot() {
        val layout = createLayout()
        val source = createProgramSource(layout)
        val contract = MountContract.build(layout, source)
        for (target in contract.guestTargets) {
            assertNotEquals("/", target)
        }
    }

    @Test
    fun build_mountOrderIsDeterministic() {
        val layout = createLayout()
        val source = createProgramSource(layout)
        val contract1 = MountContract.build(layout, source)
        val contract2 = MountContract.build(layout, source)
        assertEquals(contract1.mounts, contract2.mounts)
        assertEquals(contract1.guestTargets, contract2.guestTargets)
    }

    @Test
    fun build_differentHostLayoutsSameGuestTargets() {
        val layout1 = createLayout()
        val layout2 = createLayout()
        val source1 = createProgramSource(layout1)
        val source2 = createProgramSource(layout2)
        val contract1 = MountContract.build(layout1, source1)
        val contract2 = MountContract.build(layout2, source2)
        assertEquals(contract1.guestTargets, contract2.guestTargets)
    }

    @Test
    fun build_differentVersionsSameGuestTargets() {
        val layout = createLayout()
        val source1 = createProgramSource(layout, "1.0.0")
        val source2 = createProgramSource(layout, "2.0.0")
        val contract1 = MountContract.build(layout, source1)
        val contract2 = MountContract.build(layout, source2)
        assertEquals(contract1.guestTargets, contract2.guestTargets)
        assertNotEquals(contract1.programMount.hostSource, contract2.programMount.hostSource)
    }

    @Test
    fun toProotBindMounts_preservesOrder() {
        val layout = createLayout()
        val source = createProgramSource(layout)
        val contract = MountContract.build(layout, source)
        val bindMounts = contract.toProotBindMounts()
        assertEquals(7, bindMounts.size)
        assertEquals(GuestLayout.PROGRAM, bindMounts[0].guest)
        assertEquals(GuestLayout.CONFIG, bindMounts[1].guest)
        assertEquals(GuestLayout.DATA, bindMounts[2].guest)
        assertEquals(GuestLayout.CACHE, bindMounts[3].guest)
        assertEquals(GuestLayout.LOGS, bindMounts[4].guest)
        assertEquals(GuestLayout.RUN, bindMounts[5].guest)
        assertEquals(GuestLayout.HOME, bindMounts[6].guest)
    }

    @Test
    fun programMount_hostSourceUsesActiveVersion() {
        val layout = createLayout()
        val source = createProgramSource(layout, "3.5.0")
        val contract = MountContract.build(layout, source)
        assertTrue(
            "Program source must be within versions/3.5.0",
            unixPath(File(contract.programMount.hostSource)).contains("versions/3.5.0")
        )
    }

    @Test
    fun eachRole_hasDistinctGuestTarget() {
        val layout = createLayout()
        val source = createProgramSource(layout)
        val contract = MountContract.build(layout, source)
        val roleToGuest = contract.mounts.associate { it.role to it.guestTarget }
        assertEquals(7, roleToGuest.values.toSet().size)
    }
}
