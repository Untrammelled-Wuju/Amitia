package com.amitia.amitia_app.runtime.install

import com.amitia.amitia_app.runtime.install.internal.DefaultRuntimeInstallPaths
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

class InstallTransactionTest {

    @get:Rule
    val tempFolder = TemporaryFolder()

    @Test
    fun transaction_startsInCreatedState() {
        val baseDir = tempFolder.newFolder("txn-created")
        val paths = DefaultRuntimeInstallPaths(baseDir)
        val stagingDir = paths.stagingDir("txn-1")
        val journalFile = File(paths.transactionsRoot(), "txn-1.journal")

        val transaction = DefaultInstallTransaction(
            journalFile = journalFile,
            stagingDir = stagingDir,
        )

        assertEquals(TransactionStage.CREATED, transaction.stage())
        assertTrue(transaction.transactionId.isNotEmpty())
        assertEquals(36, transaction.transactionId.length)
    }

    @Test
    fun transaction_updatesStageCorrectly() {
        val baseDir = tempFolder.newFolder("txn-update-stage")
        val paths = DefaultRuntimeInstallPaths(baseDir)
        val transaction = DefaultInstallTransaction(
            journalFile = File(paths.transactionsRoot(), "txn-2.journal"),
            stagingDir = paths.stagingDir("txn-2"),
        )

        transaction.updateStage(TransactionStage.PACKAGE_VERIFIED)
        assertEquals(TransactionStage.PACKAGE_VERIFIED, transaction.stage())

        transaction.updateStage(TransactionStage.ROOTFS_PREPARED)
        assertEquals(TransactionStage.ROOTFS_PREPARED, transaction.stage())

        transaction.updateStage(TransactionStage.RUNTIME_EXTRACTED)
        assertEquals(TransactionStage.RUNTIME_EXTRACTED, transaction.stage())

        transaction.updateStage(TransactionStage.RUNTIME_VERIFIED)
        assertEquals(TransactionStage.RUNTIME_VERIFIED, transaction.stage())

        transaction.updateStage(TransactionStage.PUBLISHED)
        assertEquals(TransactionStage.PUBLISHED, transaction.stage())

        transaction.updateStage(TransactionStage.ACTIVATED)
        assertEquals(TransactionStage.ACTIVATED, transaction.stage())

        transaction.updateStage(TransactionStage.COMPLETED)
        assertEquals(TransactionStage.COMPLETED, transaction.stage())
    }

    @Test
    fun transaction_persistsJournal() {
        val baseDir = tempFolder.newFolder("txn-persist")
        val paths = DefaultRuntimeInstallPaths(baseDir)
        val journalFile = File(paths.transactionsRoot(), "txn-3.journal")

        val transaction = DefaultInstallTransaction(
            journalFile = journalFile,
            stagingDir = paths.stagingDir("txn-3"),
        )

        transaction.setRuntimeVersion("1.0.0")
        transaction.setPackageSha256("sha256-hex-64-chars-0000000000000000000000000000")
        transaction.setTargetVersionDir("/path/to/version")
        transaction.updateStage(TransactionStage.PACKAGE_VERIFIED)

        assertTrue(journalFile.exists())

        val journal = transaction.getJournal()
        assertEquals(TransactionStage.PACKAGE_VERIFIED, journal.stage)
        assertEquals("1.0.0", journal.runtimeVersion)
        assertEquals("sha256-hex-64-chars-0000000000000000000000000000", journal.packageSha256)
        assertEquals("/path/to/version", journal.targetVersionDir)
        assertTrue(journal.createdAtEpochMillis > 0)
        assertTrue(journal.updatedAtEpochMillis >= journal.createdAtEpochMillis)
    }

    @Test
    fun transaction_rollback_removesStagingDir() {
        val baseDir = tempFolder.newFolder("txn-rollback")
        val paths = DefaultRuntimeInstallPaths(baseDir)
        val stagingDir = File(paths.stagingDir("txn-4"))
        stagingDir.mkdirs()
        File(stagingDir, "test-file.txt").writeText("test content")
        File(stagingDir, "subdir").mkdirs()
        File(stagingDir, "subdir/another.txt").writeText("more content")

        assertTrue(stagingDir.exists())
        assertTrue(stagingDir.listFiles()?.isNotEmpty() == true)

        val transaction = DefaultInstallTransaction(
            journalFile = File(paths.transactionsRoot(), "txn-4.journal"),
            stagingDir = stagingDir.absolutePath,
        )
        transaction.rollback()

        assertFalse(stagingDir.exists())
    }

    @Test
    fun transaction_returnsImmutableJournal() {
        val baseDir = tempFolder.newFolder("txn-immutable")
        val paths = DefaultRuntimeInstallPaths(baseDir)
        val transaction = DefaultInstallTransaction(
            journalFile = File(paths.transactionsRoot(), "txn-5.journal"),
            stagingDir = paths.stagingDir("txn-5"),
        )

        transaction.setRuntimeVersion("1.0.0")
        val journal1 = transaction.getJournal()

        transaction.updateStage(TransactionStage.PACKAGE_VERIFIED)
        val journal2 = transaction.getJournal()

        assertTrue(journal1 !== journal2)
        assertEquals(TransactionStage.CREATED, journal1.stage)
        assertEquals(TransactionStage.PACKAGE_VERIFIED, journal2.stage)
    }
}
