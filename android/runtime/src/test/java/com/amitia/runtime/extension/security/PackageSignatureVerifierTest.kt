package com.amitia.runtime.extension.security

import org.bouncycastle.crypto.params.Ed25519PrivateKeyParameters
import org.bouncycastle.crypto.signers.Ed25519Signer
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.security.SecureRandom
import java.time.Instant

class PackageSignatureVerifierTest {

    private data class KeyMaterial(val publicKey: ByteArray, val privateKey: ByteArray)

    private fun generateKeyPair(): KeyMaterial {
        val privateKeyParams = Ed25519PrivateKeyParameters(SecureRandom())
        val publicKeyParams = privateKeyParams.generatePublicKey()
        return KeyMaterial(publicKeyParams.encoded, privateKeyParams.encoded)
    }

    private fun sign(privateKey: ByteArray, message: ByteArray): ByteArray {
        val privateKeyParams = Ed25519PrivateKeyParameters(privateKey, 0)
        val signer = Ed25519Signer()
        signer.init(true, privateKeyParams)
        signer.update(message, 0, message.size)
        return signer.generateSignature()
    }

    private fun buildKey(
        publisherId: String,
        publicKey: ByteArray,
        keyId: String = PackageSignatureVerifier.computeKeyId(publicKey),
        createdAt: Instant = Instant.now(),
        expiresAt: Instant? = null,
        state: KeyState = KeyState.ACTIVE
    ): PublisherKey {
        return PublisherKey(
            keyId = keyId,
            publisherId = publisherId,
            publicKey = publicKey,
            algorithm = "ed25519",
            createdAt = createdAt,
            expiresAt = expiresAt,
            state = state
        )
    }

    private fun buildIdentity(
        publisherId: String,
        key: PublisherKey,
        trustLevel: TrustLevel = TrustLevel.TRUSTED
    ): PublisherIdentity {
        return PublisherIdentity(
            publisherId = publisherId,
            displayName = publisherId,
            keys = listOf(key),
            trustLevel = trustLevel
        )
    }

    private fun buildSignedDoc(
        publisherId: String,
        keyId: String,
        treeHash: String,
        privateKey: ByteArray,
        algorithm: String = "ed25519"
    ): SignatureDoc {
        val message = PackageSignatureVerifier.buildSignatureMessage(publisherId, treeHash)
            .toByteArray(Charsets.UTF_8)
        val signature = sign(privateKey, message)
        return SignatureDoc(
            algorithm = algorithm,
            keyId = keyId,
            signature = signature,
            publisherId = publisherId
        )
    }

    @Test
    fun verify_validSignature_passes() {
        val km = generateKeyPair()
        val publisherId = "test-publisher"
        val keyId = PackageSignatureVerifier.computeKeyId(km.publicKey)
        val treeHash = "dummy-tree-hash"

        val trustStore = InMemoryPublisherTrustStore()
        val key = buildKey(publisherId, km.publicKey, keyId)
        trustStore.registerPublisher(buildIdentity(publisherId, key))

        val verifier = PackageSignatureVerifier(trustStore = trustStore)
        val sigDoc = buildSignedDoc(publisherId, keyId, treeHash, km.privateKey)

        val result = verifier.verify(sigDoc, treeHash)

        assertTrue(result.verified)
        assertEquals(publisherId, result.publisherId)
        assertEquals(keyId, result.keyId)
        assertNotNull(result.fingerprint)
        assertEquals(key.fingerprint(), result.fingerprint)
    }

    @Test
    fun verify_tamperedSignature_fails() {
        val km = generateKeyPair()
        val publisherId = "test-publisher"
        val keyId = PackageSignatureVerifier.computeKeyId(km.publicKey)
        val treeHash = "dummy-tree-hash"

        val trustStore = InMemoryPublisherTrustStore()
        val key = buildKey(publisherId, km.publicKey, keyId)
        trustStore.registerPublisher(buildIdentity(publisherId, key))

        val sigDoc = buildSignedDoc(publisherId, keyId, treeHash, km.privateKey)
        val tampered = sigDoc.signature.copyOf()
        tampered[0] = (tampered[0].toInt() xor 0xFF).toByte()
        val tamperedDoc = sigDoc.copy(signature = tampered)

        val verifier = PackageSignatureVerifier(trustStore = trustStore)
        val result = verifier.verify(tamperedDoc, treeHash)

        assertFalse(result.verified)
        assertEquals("signature verification failed", result.error)
    }

    @Test
    fun verify_nullSignature_returnsFalse() {
        val verifier = PackageSignatureVerifier(trustStore = InMemoryPublisherTrustStore())
        val result = verifier.verify(null, "tree-hash")
        assertFalse(result.verified)
        assertEquals("signature missing", result.error)
    }

    @Test
    fun verify_unsupportedAlgorithm_returnsFalse() {
        val sigDoc = SignatureDoc(
            algorithm = "rsa",
            keyId = "key-1",
            signature = ByteArray(64),
            publisherId = "publisher-1"
        )
        val verifier = PackageSignatureVerifier(trustStore = InMemoryPublisherTrustStore())
        val result = verifier.verify(sigDoc, "tree-hash")
        assertFalse(result.verified)
        assertTrue(result.error!!.contains("unsupported signature algorithm"))
    }

    @Test
    fun verify_emptyTreeHash_returnsFalse() {
        val sigDoc = SignatureDoc(
            algorithm = "ed25519",
            keyId = "key-1",
            signature = ByteArray(64),
            publisherId = "publisher-1"
        )
        val verifier = PackageSignatureVerifier(trustStore = InMemoryPublisherTrustStore())
        val result = verifier.verify(sigDoc, "")
        assertFalse(result.verified)
        assertEquals("content tree hash empty", result.error)
    }

    @Test
    fun verify_emptyPublisherId_returnsFalse() {
        val sigDoc = SignatureDoc(
            algorithm = "ed25519",
            keyId = "key-1",
            signature = ByteArray(64),
            publisherId = null
        )
        val verifier = PackageSignatureVerifier(trustStore = InMemoryPublisherTrustStore())
        val result = verifier.verify(sigDoc, "tree-hash")
        assertFalse(result.verified)
        assertEquals("publisher ID missing in signature", result.error)
    }

    @Test
    fun verify_emptyKeyId_returnsFalse() {
        val sigDoc = SignatureDoc(
            algorithm = "ed25519",
            keyId = "",
            signature = ByteArray(64),
            publisherId = "publisher-1"
        )
        val verifier = PackageSignatureVerifier(trustStore = InMemoryPublisherTrustStore())
        val result = verifier.verify(sigDoc, "tree-hash")
        assertFalse(result.verified)
        assertEquals("key ID missing in signature", result.error)
    }

    @Test
    fun verify_noTrustStore_returnsFalse() {
        val sigDoc = SignatureDoc(
            algorithm = "ed25519",
            keyId = "key-1",
            signature = ByteArray(64),
            publisherId = "publisher-1"
        )
        val verifier = PackageSignatureVerifier()
        val result = verifier.verify(sigDoc, "tree-hash")
        assertFalse(result.verified)
        assertEquals("no trust store available", result.error)
    }

    @Test
    fun verify_unknownPublisher_returnsFalse() {
        val km = generateKeyPair()
        val publisherId = "unknown-publisher"
        val keyId = PackageSignatureVerifier.computeKeyId(km.publicKey)

        val trustStore = InMemoryPublisherTrustStore()
        val verifier = PackageSignatureVerifier(trustStore = trustStore)
        val sigDoc = SignatureDoc(
            algorithm = "ed25519",
            keyId = keyId,
            signature = ByteArray(64),
            publisherId = publisherId
        )

        val result = verifier.verify(sigDoc, "tree-hash")
        assertFalse(result.verified)
        assertTrue(result.error!!.contains("unknown publisher"))
    }

    @Test
    fun verify_blockedPublisher_returnsFalse() {
        val km = generateKeyPair()
        val publisherId = "blocked-publisher"
        val keyId = PackageSignatureVerifier.computeKeyId(km.publicKey)

        val trustStore = InMemoryPublisherTrustStore()
        val key = buildKey(publisherId, km.publicKey, keyId)
        trustStore.registerPublisher(buildIdentity(publisherId, key, TrustLevel.BLOCKED))

        val verifier = PackageSignatureVerifier(trustStore = trustStore)
        val sigDoc = SignatureDoc(
            algorithm = "ed25519",
            keyId = keyId,
            signature = ByteArray(64),
            publisherId = publisherId
        )

        val result = verifier.verify(sigDoc, "tree-hash")
        assertFalse(result.verified)
        assertTrue(result.error!!.contains("publisher blocked"))
    }

    @Test
    fun verify_revokedKey_returnsFalse() {
        val km = generateKeyPair()
        val publisherId = "test-publisher"
        val keyId = PackageSignatureVerifier.computeKeyId(km.publicKey)

        val trustStore = InMemoryPublisherTrustStore()
        val key = buildKey(publisherId, km.publicKey, keyId)
        trustStore.registerPublisher(buildIdentity(publisherId, key))

        val revocationList = RevocationList()
        revocationList.revoke(
            RevocationEntry(
                publisherId = publisherId,
                keyId = keyId,
                reason = "compromised",
                revokedAt = Instant.now()
            )
        )

        val verifier = PackageSignatureVerifier(
            trustStore = trustStore,
            revocationList = revocationList
        )
        val sigDoc = SignatureDoc(
            algorithm = "ed25519",
            keyId = keyId,
            signature = ByteArray(64),
            publisherId = publisherId
        )

        val result = verifier.verify(sigDoc, "tree-hash")
        assertFalse(result.verified)
        assertTrue(result.error!!.contains("key revoked"))
    }

    @Test
    fun verify_expiredKey_returnsFalse() {
        val km = generateKeyPair()
        val publisherId = "test-publisher"
        val keyId = PackageSignatureVerifier.computeKeyId(km.publicKey)

        val key = PublisherKey(
            keyId = keyId,
            publisherId = publisherId,
            publicKey = km.publicKey,
            algorithm = "ed25519",
            createdAt = Instant.now().minusSeconds(7200),
            expiresAt = Instant.now().minusSeconds(3600),
            state = KeyState.ACTIVE
        )

        val trustStore = InMemoryPublisherTrustStore()
        trustStore.registerPublisher(buildIdentity(publisherId, key))

        val verifier = PackageSignatureVerifier(trustStore = trustStore)
        val sigDoc = SignatureDoc(
            algorithm = "ed25519",
            keyId = keyId,
            signature = ByteArray(64),
            publisherId = publisherId
        )

        val result = verifier.verify(sigDoc, "tree-hash")
        assertFalse(result.verified)
        assertEquals("key expired", result.error)
    }

    @Test
    fun verifyWithPublicKey_validSignature_returnsTrue() {
        val km = generateKeyPair()
        val publisherId = "test-publisher"
        val treeHash = "dummy-tree-hash"

        val sigDoc = buildSignedDoc(publisherId, "any-key", treeHash, km.privateKey)

        val verifier = PackageSignatureVerifier()
        val result = verifier.verifyWithPublicKey(sigDoc, treeHash, km.publicKey)
        assertTrue(result)
    }

    @Test
    fun verifyWithPublicKey_invalidSignature_returnsFalse() {
        val km = generateKeyPair()
        val publisherId = "test-publisher"
        val treeHash = "dummy-tree-hash"

        val sigDoc = SignatureDoc(
            algorithm = "ed25519",
            keyId = "any-key",
            signature = ByteArray(64),
            publisherId = publisherId
        )

        val verifier = PackageSignatureVerifier()
        val result = verifier.verifyWithPublicKey(sigDoc, treeHash, km.publicKey)
        assertFalse(result)
    }

    @Test
    fun computeKeyId_returnsSha256PrefixedHash() {
        val km = generateKeyPair()
        val keyId = PackageSignatureVerifier.computeKeyId(km.publicKey)

        assertTrue(keyId.startsWith("sha256:"))
        val hexPart = keyId.removePrefix("sha256:")
        assertEquals(64, hexPart.length)

        val key = buildKey("any-publisher", km.publicKey, keyId)
        assertEquals(key.fingerprint(), keyId)
    }
}
