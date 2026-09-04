package com.amitia.amitia_app.runtime.install

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

class PathValidatorTest {

    @get:Rule
    val tempFolder = TemporaryFolder()

    @Test
    fun isValidRuntimeVersion_acceptsValidVersions() {
        assertTrue(PathValidator.isValidRuntimeVersion("1.0.0"))
        assertTrue(PathValidator.isValidRuntimeVersion("1.0.0-beta.1"))
        assertTrue(PathValidator.isValidRuntimeVersion("2.1.3"))
        assertTrue(PathValidator.isValidRuntimeVersion("1.0.0-rc.1+build.123"))
    }

    @Test
    fun isValidRuntimeVersion_rejectsInvalidVersions() {
        assertFalse(PathValidator.isValidRuntimeVersion(""))
        assertFalse(PathValidator.isValidRuntimeVersion(".."))
        assertFalse(PathValidator.isValidRuntimeVersion("../etc/passwd"))
        assertFalse(PathValidator.isValidRuntimeVersion("/absolute/path"))
        assertFalse(PathValidator.isValidRuntimeVersion("\\windows\\path"))
        assertFalse(PathValidator.isValidRuntimeVersion("1.0.0/../../etc"))
        assertFalse(PathValidator.isValidRuntimeVersion("version with spaces"))
        assertFalse(PathValidator.isValidRuntimeVersion("version<script>"))
        assertFalse(PathValidator.isValidRuntimeVersion("latest"))
        assertFalse(PathValidator.isValidRuntimeVersion("current"))
        assertFalse(PathValidator.isValidRuntimeVersion("stable"))
    }

    @Test
    fun isWithin_acceptsDirectChild() {
        val parent = tempFolder.newFolder("parent")
        val child = File(parent, "child")
        child.mkdirs()
        assertTrue(PathValidator.isWithin(child, parent))
    }

    @Test
    fun isWithin_acceptsNestedChild() {
        val parent = tempFolder.newFolder("parent")
        val nested = File(File(parent, "child"), "grandchild")
        nested.mkdirs()
        assertTrue(PathValidator.isWithin(nested, parent))
    }

    @Test
    fun isWithin_rejectsDifferentTrees() {
        val parent = tempFolder.newFolder("parent")
        val other = tempFolder.newFolder("other")
        assertFalse(PathValidator.isWithin(other, parent))
    }

    @Test
    fun isWithin_rejectsSimilarPrefixAttack() {
        val runtime1 = tempFolder.newFolder("runtime")
        val runtime2 = File(tempFolder.root, "runtime2")
        runtime2.mkdirs()
        assertFalse(PathValidator.isWithin(runtime2, runtime1))
    }

    @Test
    fun isWithin_acceptsSamePath() {
        val dir = tempFolder.newFolder("same")
        assertTrue(PathValidator.isWithin(dir, dir))
    }
}
