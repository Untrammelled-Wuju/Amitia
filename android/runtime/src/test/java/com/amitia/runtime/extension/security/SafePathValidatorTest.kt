package com.amitia.runtime.extension.security

import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test

class SafePathValidatorTest {

    private val validator = SafePathValidator()

    @Test
    fun normalizeArchivePath_acceptsNormalRelativePath() {
        assertEquals("modules/main/index.js", validator.normalizeArchivePath("modules/main/index.js"))
    }

    @Test
    fun normalizeArchivePath_rejectsLeadingParentTraversal() {
        assertThrows(PathTraversalException::class.java) {
            validator.normalizeArchivePath("../etc/passwd")
        }
    }

    @Test
    fun normalizeArchivePath_rejectsEmbeddedParentTraversal() {
        assertThrows(PathTraversalException::class.java) {
            validator.normalizeArchivePath("modules/../../etc/passwd")
        }
    }

    @Test
    fun normalizeArchivePath_rejectsUnixAbsolutePath() {
        assertThrows(AbsolutePathException::class.java) {
            validator.normalizeArchivePath("/etc/passwd")
        }
    }

    @Test
    fun normalizeArchivePath_rejectsWindowsDrivePath() {
        assertThrows(AbsolutePathException::class.java) {
            validator.normalizeArchivePath("C:/Windows/system32")
        }
    }

    @Test
    fun normalizeArchivePath_rejectsUncPath() {
        assertThrows(AbsolutePathException::class.java) {
            validator.normalizeArchivePath("\\\\server\\share")
        }
    }

    @Test
    fun normalizeArchivePath_convertsBackslashesToSlashes() {
        assertEquals("modules/main/index.js", validator.normalizeArchivePath("modules\\main\\index.js"))
    }

    @Test
    fun normalizeArchivePath_rejectsTooLongPath() {
        assertThrows(PathTooLongException::class.java) {
            validator.normalizeArchivePath("a".repeat(513))
        }
    }

    @Test
    fun normalizeArchivePath_rejectsExcessiveDirectoryDepth() {
        val deep = List(33) { "a" }.joinToString("/")
        assertThrows(PathDepthExceededException::class.java) {
            validator.normalizeArchivePath(deep)
        }
    }

    @Test
    fun normalizeArchivePath_rejectsWindowsReservedNameCon() {
        assertThrows(WindowsReservedNameException::class.java) {
            validator.normalizeArchivePath("con")
        }
    }

    @Test
    fun normalizeArchivePath_rejectsWindowsReservedNameNulTxt() {
        assertThrows(WindowsReservedNameException::class.java) {
            validator.normalizeArchivePath("nul.txt")
        }
    }

    @Test
    fun validatePath_rejectsDoubleSlashStructure() {
        assertThrows(InvalidStructureException::class.java) {
            validator.validatePath("modules//main")
        }
    }

    @Test
    fun detectCollisions_reportsCaseInsensitiveCollision() {
        val collisions = validator.detectCollisions(listOf("Modules/main", "modules/main"))
        assertEquals(1, collisions.size)
        assertEquals("Modules/main", collisions[0].pathA)
        assertEquals("modules/main", collisions[0].pathB)
        assertEquals("case_insensitive_collision", collisions[0].reason)
    }

    @Test
    fun resolveWithinRoot_resolvesNestedPathUnderRoot() {
        assertEquals(
            "base/modules/main/index.js",
            validator.resolveWithinRoot("base", "modules/main/index.js")
        )
    }

    @Test
    fun resolveWithinRoot_rejectsPathEscapingRoot() {
        assertThrows(PathTraversalException::class.java) {
            validator.resolveWithinRoot("base", "../../etc/passwd")
        }
    }
}
