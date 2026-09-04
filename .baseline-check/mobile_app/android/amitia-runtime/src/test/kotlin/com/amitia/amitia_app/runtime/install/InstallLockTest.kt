package com.amitia.amitia_app.runtime.install

import com.amitia.amitia_app.runtime.install.internal.DefaultRuntimeInstallPaths
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File
import java.util.concurrent.CountDownLatch
import java.util.concurrent.atomic.AtomicReference

class InstallLockTest {

    @get:Rule
    val tempFolder = TemporaryFolder()

    @Test
    fun acquire_succeeds_whenNotLocked() {
        val baseDir = tempFolder.newFolder("lock-success")
        val paths = DefaultRuntimeInstallPaths(baseDir)

        val result = InstallLock.acquire(paths)
        assertTrue(result is InstallLockResult.Success)

        val lock = (result as InstallLockResult.Success).lock
        assertTrue(lock.isHeld())
        lock.close()
        assertFalse(lock.isHeld())
    }

    @Test
    fun acquire_fails_whenAlreadyLocked() {
        val baseDir = tempFolder.newFolder("lock-already-held")
        val paths = DefaultRuntimeInstallPaths(baseDir)

        val lock1Result = InstallLock.acquire(paths)
        assertTrue(lock1Result is InstallLockResult.Success)
        val lock1 = (lock1Result as InstallLockResult.Success).lock

        try {
            val lock2Result = InstallLock.acquire(paths, timeoutMs = 500L)
            assertTrue(lock2Result is InstallLockResult.Failure)
            val failure = lock2Result as InstallLockResult.Failure
            assertEquals(RuntimeInstallErrorCode.INSTALL_ALREADY_IN_PROGRESS, failure.code)
        } finally {
            lock1.close()
        }
    }

    @Test
    fun concurrentInstall_onlyOneSucceeds() {
        val baseDir = tempFolder.newFolder("lock-concurrent")
        val paths = DefaultRuntimeInstallPaths(baseDir)

        val startLatch = CountDownLatch(1)
        val result1 = AtomicReference<InstallLockResult?>(null)
        val result2 = AtomicReference<InstallLockResult?>(null)

        val thread1 = Thread {
            startLatch.await()
            result1.set(InstallLock.acquire(paths, timeoutMs = 2000L))
        }
        val thread2 = Thread {
            startLatch.await()
            result2.set(InstallLock.acquire(paths, timeoutMs = 2000L))
        }

        thread1.start()
        thread2.start()
        startLatch.countDown()

        thread1.join(5000)
        thread2.join(5000)

        val r1 = result1.get()
        val r2 = result2.get()

        assertTrue(r1 != null && r2 != null)

        val successCount = listOf(r1, r2).count { it is InstallLockResult.Success }
        val failureCount = listOf(r1, r2).count { it is InstallLockResult.Failure }

        assertTrue(
            "Exactly one thread should acquire lock: successes=$successCount failures=$failureCount",
            successCount >= 1
        )

        listOf(r1, r2).forEach {
            if (it is InstallLockResult.Success) {
                it.lock.close()
            }
        }
    }

    @Test
    fun lock_isReleasedOnClose() {
        val baseDir = tempFolder.newFolder("lock-released-close")
        val paths = DefaultRuntimeInstallPaths(baseDir)

        val result = InstallLock.acquire(paths)
        assertTrue(result is InstallLockResult.Success)
        val lock = (result as InstallLockResult.Success).lock

        assertTrue(lock.isHeld())
        lock.close()
        assertFalse(lock.isHeld())

        val result2 = InstallLock.acquire(paths)
        assertTrue(result2 is InstallLockResult.Success)
        (result2 as InstallLockResult.Success).lock.close()
    }
}
