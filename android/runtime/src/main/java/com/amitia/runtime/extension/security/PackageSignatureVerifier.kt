package com.amitia.runtime.extension.security

import org.bouncycastle.crypto.params.Ed25519PublicKeyParameters
import org.bouncycastle.crypto.signers.Ed25519Signer
import java.security.MessageDigest

class PackageSignatureVerifier(
    private val trustStore: PublisherTrustStore? = null,
    private val revocationList: RevocationList? = null
) {
    data class SignatureVerificationResult(
        val verified: Boolean,
        val publisherId: String?,
        val keyId: String?,
        val fingerprint: String?,
        val error: String? = null
    )

    fun verify(
        signature: SignatureDoc?,
        treeHash: String,
        manifestHash: String? = null
    ): SignatureVerificationResult {
        if (signature == null) {
            return SignatureVerificationResult(
                verified = false,
                publisherId = null,
                keyId = null,
                fingerprint = null,
                error = "signature missing"
            )
        }

        if (signature.algorithm != "ed25519") {
            return SignatureVerificationResult(
                verified = false,
                publisherId = signature.publisherId,
                keyId = signature.keyId,
                fingerprint = null,
                error = "unsupported signature algorithm: ${signature.algorithm}"
            )
        }

        if (treeHash.isEmpty()) {
            return SignatureVerificationResult(
                verified = false,
                publisherId = signature.publisherId,
                keyId = signature.keyId,
                fingerprint = null,
                error = "content tree hash empty"
            )
        }

        val publisherId = signature.publisherId
        val keyId = signature.keyId

        if (publisherId.isNullOrEmpty()) {
            return SignatureVerificationResult(
                verified = false,
                publisherId = null,
                keyId = keyId,
                fingerprint = null,
                error = "publisher ID missing in signature"
            )
        }

        if (keyId.isNullOrEmpty()) {
            return SignatureVerificationResult(
                verified = false,
                publisherId = publisherId,
                keyId = null,
                fingerprint = null,
                error = "key ID missing in signature"
            )
        }

        val trustStore = this.trustStore
        if (trustStore == null) {
            return SignatureVerificationResult(
                verified = false,
                publisherId = publisherId,
                keyId = keyId,
                fingerprint = null,
                error = "no trust store available"
            )
        }

        if (trustStore.isPublisherBlocked(publisherId)) {
            return SignatureVerificationResult(
                verified = false,
                publisherId = publisherId,
                keyId = keyId,
                fingerprint = null,
                error = "publisher blocked: $publisherId"
            )
        }

        val revocationList = this.revocationList
        if (revocationList != null) {
            val keyRevocation = revocationList.checkKey(publisherId, keyId)
            if (keyRevocation != null) {
                return SignatureVerificationResult(
                    verified = false,
                    publisherId = publisherId,
                    keyId = keyId,
                    fingerprint = null,
                    error = "key revoked: ${keyRevocation.reason}"
                )
            }

            val publisherRevocation = revocationList.checkPublisher(publisherId)
            if (publisherRevocation != null) {
                return SignatureVerificationResult(
                    verified = false,
                    publisherId = publisherId,
                    keyId = keyId,
                    fingerprint = null,
                    error = "publisher revoked: ${publisherRevocation.reason}"
                )
            }
        }

        val publisher = trustStore.getPublisher(publisherId)
        if (publisher == null) {
            return SignatureVerificationResult(
                verified = false,
                publisherId = publisherId,
                keyId = keyId,
                fingerprint = null,
                error = "unknown publisher: $publisherId"
            )
        }

        if (!publisher.trustLevel.allowsInstallation()) {
            return SignatureVerificationResult(
                verified = false,
                publisherId = publisherId,
                keyId = keyId,
                fingerprint = null,
                error = "trust level does not allow installation: ${publisher.trustLevel}"
            )
        }

        val key = publisher.findKey(keyId)
        if (key == null) {
            return SignatureVerificationResult(
                verified = false,
                publisherId = publisherId,
                keyId = keyId,
                fingerprint = null,
                error = "key not found: $keyId"
            )
        }

        if (key.isRevoked()) {
            return SignatureVerificationResult(
                verified = false,
                publisherId = publisherId,
                keyId = keyId,
                fingerprint = key.fingerprint(),
                error = "key revoked"
            )
        }

        if (key.isExpired()) {
            return SignatureVerificationResult(
                verified = false,
                publisherId = publisherId,
                keyId = keyId,
                fingerprint = key.fingerprint(),
                error = "key expired"
            )
        }

        if (!key.isUsable()) {
            return SignatureVerificationResult(
                verified = false,
                publisherId = publisherId,
                keyId = keyId,
                fingerprint = key.fingerprint(),
                error = "key not usable: ${key.state}"
            )
        }

        if (key.publicKey.size != ED25519_PUBLIC_KEY_SIZE) {
            return SignatureVerificationResult(
                verified = false,
                publisherId = publisherId,
                keyId = keyId,
                fingerprint = key.fingerprint(),
                error = "invalid public key size: ${key.publicKey.size}"
            )
        }

        val message = buildSignatureMessage(publisherId, treeHash)
        val isValid = verifyEd25519(
            publicKey = key.publicKey,
            message = message.toByteArray(Charsets.UTF_8),
            signature = signature.signature
        )

        if (!isValid) {
            return SignatureVerificationResult(
                verified = false,
                publisherId = publisherId,
                keyId = keyId,
                fingerprint = key.fingerprint(),
                error = "signature verification failed"
            )
        }

        return SignatureVerificationResult(
            verified = true,
            publisherId = publisherId,
            keyId = keyId,
            fingerprint = key.fingerprint(),
            error = null
        )
    }

    fun verifyWithPublicKey(
        signature: SignatureDoc?,
        treeHash: String,
        publicKey: ByteArray
    ): Boolean {
        if (signature == null) return false
        if (signature.algorithm != "ed25519") return false
        if (treeHash.isEmpty()) return false
        if (publicKey.size != ED25519_PUBLIC_KEY_SIZE) return false

        val publisherId = signature.publisherId ?: return false
        val message = buildSignatureMessage(publisherId, treeHash)
        return verifyEd25519(
            publicKey = publicKey,
            message = message.toByteArray(Charsets.UTF_8),
            signature = signature.signature
        )
    }

    private fun verifyEd25519(
        publicKey: ByteArray,
        message: ByteArray,
        signature: ByteArray
    ): Boolean {
        return try {
            val publicKeyParams = Ed25519PublicKeyParameters(publicKey, 0)
            val verifier = Ed25519Signer()
            verifier.init(false, publicKeyParams)
            verifier.update(message, 0, message.size)
            verifier.verifySignature(signature)
        } catch (e: Exception) {
            false
        }
    }

    companion object {
        const val ED25519_PUBLIC_KEY_SIZE = 32
        const val ED25519_SIGNATURE_SIZE = 64

        fun buildSignatureMessage(publisherId: String, treeHash: String): String {
            return "$publisherId:$treeHash"
        }

        fun computeKeyId(publicKey: ByteArray): String {
            val digest = MessageDigest.getInstance("SHA-256")
            val hash = digest.digest(publicKey)
            return "sha256:" + hash.joinToString("") { "%02x".format(it) }
        }
    }
}
