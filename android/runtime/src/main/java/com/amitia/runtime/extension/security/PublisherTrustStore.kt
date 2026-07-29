package com.amitia.runtime.extension.security

import com.amitia.runtime.extension.ExtensionApiClient
import java.time.Instant
import java.util.Base64
import java.util.concurrent.ConcurrentHashMap
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive

enum class TrustLevel {
    OFFICIAL,
    TRUSTED,
    USER_TRUSTED,
    UNKNOWN,
    BLOCKED,
    REVOKED,
    DEVELOPMENT;

    fun allowsInstallation(isDebug: Boolean = false): Boolean = when (this) {
        OFFICIAL, TRUSTED, USER_TRUSTED -> true
        DEVELOPMENT -> isDebug
        else -> false
    }

    fun allowsAutoUpdate(): Boolean = this in setOf(OFFICIAL, TRUSTED, USER_TRUSTED)

    fun allowsHighRiskRuntime(): Boolean = this in setOf(OFFICIAL, TRUSTED)

    fun isBlocked(): Boolean = this in setOf(BLOCKED, REVOKED)

    companion object {
        fun fromString(value: String?): TrustLevel {
            return when (value?.lowercase()) {
                "official" -> OFFICIAL
                "trusted" -> TRUSTED
                "user_trusted", "usertrusted" -> USER_TRUSTED
                "blocked" -> BLOCKED
                "revoked" -> REVOKED
                "development", "dev" -> DEVELOPMENT
                else -> UNKNOWN
            }
        }
    }
}

enum class KeyState {
    ACTIVE,
    ROTATED,
    EXPIRED,
    REVOKED,
    COMPROMISED,
    UNKNOWN;

    fun isUsable(): Boolean = this == ACTIVE
}

data class PublisherKey(
    val keyId: String,
    val publisherId: String,
    val publicKey: ByteArray,
    val algorithm: String,
    val createdAt: Instant,
    val expiresAt: Instant? = null,
    val revokedAt: Instant? = null,
    val state: KeyState = KeyState.ACTIVE
) {
    fun isExpired(): Boolean {
        val exp = expiresAt ?: return false
        return Instant.now().isAfter(exp)
    }

    fun isRevoked(): Boolean {
        return state == KeyState.REVOKED || state == KeyState.COMPROMISED || revokedAt != null
    }

    fun isUsable(): Boolean {
        if (isExpired() || isRevoked()) return false
        return state.isUsable()
    }

    fun fingerprint(): String {
        val digest = java.security.MessageDigest.getInstance("SHA-256")
        val hash = digest.digest(publicKey)
        return "sha256:" + hash.joinToString("") { "%02x".format(it) }
    }

    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (other !is PublisherKey) return false
        return keyId == other.keyId &&
            publisherId == other.publisherId &&
            publicKey.contentEquals(other.publicKey)
    }

    override fun hashCode(): Int {
        var result = keyId.hashCode()
        result = 31 * result + publisherId.hashCode()
        result = 31 * result + publicKey.contentHashCode()
        return result
    }
}

data class PublisherIdentity(
    val publisherId: String,
    val displayName: String,
    val keys: List<PublisherKey>,
    val trustLevel: TrustLevel,
    val contact: String? = null,
    val website: String? = null
) {
    fun activeKey(): PublisherKey? = keys.firstOrNull { it.isUsable() }

    fun findKey(keyId: String): PublisherKey? = keys.firstOrNull { it.keyId == keyId }
}

interface PublisherTrustStore {
    fun getPublisher(publisherId: String): PublisherIdentity?

    fun getKey(publisherId: String, keyId: String): PublisherKey? {
        return getPublisher(publisherId)?.findKey(keyId)
    }

    fun isPublisherBlocked(publisherId: String): Boolean {
        val publisher = getPublisher(publisherId) ?: return false
        return publisher.trustLevel.isBlocked()
    }

    fun isKeyRevoked(publisherId: String, keyId: String): Boolean {
        val key = getKey(publisherId, keyId) ?: return false
        return key.isRevoked()
    }

    fun isKeyExpired(publisherId: String, keyId: String): Boolean {
        val key = getKey(publisherId, keyId) ?: return false
        return key.isExpired()
    }

    suspend fun syncFromBackend() {}
}

class InMemoryPublisherTrustStore : PublisherTrustStore {
    private val publishers = mutableMapOf<String, PublisherIdentity>()

    fun registerPublisher(identity: PublisherIdentity) {
        publishers[identity.publisherId] = identity
    }

    fun revokeTrust(publisherId: String) {
        publishers[publisherId]?.let { existing ->
            publishers[publisherId] = existing.copy(trustLevel = TrustLevel.REVOKED)
        }
    }

    fun blockPublisher(publisherId: String) {
        publishers[publisherId]?.let { existing ->
            publishers[publisherId] = existing.copy(trustLevel = TrustLevel.BLOCKED)
        }
    }

    fun revokeKey(publisherId: String, keyId: String) {
        publishers[publisherId]?.let { publisher ->
            val updatedKeys = publisher.keys.map { key ->
                if (key.keyId == keyId) {
                    key.copy(state = KeyState.REVOKED, revokedAt = Instant.now())
                } else {
                    key
                }
            }
            publishers[publisherId] = publisher.copy(keys = updatedKeys)
        }
    }

    override fun getPublisher(publisherId: String): PublisherIdentity? {
        return publishers[publisherId]
    }

    fun clear() {
        publishers.clear()
    }
}

data class RevocationEntry(
    val publisherId: String,
    val keyId: String,
    val reason: String,
    val revokedAt: Instant,
    val expiresAt: Instant? = null
) {
    fun isExpired(): Boolean {
        val exp = expiresAt ?: return false
        return Instant.now().isAfter(exp)
    }

    fun isActive(): Boolean = !isExpired()
}

class RevocationList {
    private val entries = mutableListOf<RevocationEntry>()

    fun revoke(entry: RevocationEntry) {
        entries.add(entry)
    }

    fun checkKey(publisherId: String, keyId: String): RevocationEntry? {
        return entries.firstOrNull {
            it.publisherId == publisherId &&
            it.keyId == keyId &&
            it.isActive()
        }
    }

    fun checkPublisher(publisherId: String): RevocationEntry? {
        return entries.firstOrNull {
            it.publisherId == publisherId &&
            it.keyId.isEmpty() &&
            it.isActive()
        }
    }

    fun clear() {
        entries.clear()
    }
}

class RemotePublisherTrustStore(
    private val apiClient: ExtensionApiClient
) : PublisherTrustStore {
    private val publishers = ConcurrentHashMap<String, PublisherIdentity>()

    override suspend fun syncFromBackend() {
        try {
            val response = apiClient.fetchTrustedPublishers()
            val arr = response["publishers"]?.jsonArray ?: return
            val updated = mutableMapOf<String, PublisherIdentity>()
            for (elem in arr) {
                val obj = elem.jsonObject
                val publisherId = obj.string("publisherId") ?: continue
                val keys = (obj["keys"]?.jsonArray ?: emptyList()).mapNotNull { keyElem ->
                    parseKey(keyElem.jsonObject, publisherId)
                }
                updated[publisherId] = PublisherIdentity(
                    publisherId = publisherId,
                    displayName = obj.string("displayName") ?: publisherId,
                    keys = keys,
                    trustLevel = TrustLevel.fromString(obj.string("trustLevel")),
                    contact = obj.string("contact"),
                    website = obj.string("website")
                )
            }
            publishers.clear()
            publishers.putAll(updated)
        } catch (e: Exception) {
            android.util.Log.e("RemotePublisherTrustStore", "syncFromBackend failed", e)
        }
    }

    private fun parseKey(obj: JsonObject, publisherId: String): PublisherKey? {
        return try {
            val publicKeyBase64 = obj.string("publicKey") ?: return null
            PublisherKey(
                keyId = obj.string("keyId") ?: "",
                publisherId = publisherId,
                publicKey = Base64.getDecoder().decode(publicKeyBase64),
                algorithm = obj.string("algorithm") ?: "ed25519",
                createdAt = Instant.parse(obj.string("createdAt") ?: Instant.now().toString()),
                expiresAt = obj.string("expiresAt")?.let { Instant.parse(it) },
                state = runCatching { KeyState.valueOf(obj.string("state") ?: "ACTIVE") }.getOrDefault(KeyState.ACTIVE)
            )
        } catch (e: Exception) {
            null
        }
    }

    private fun JsonObject.string(key: String): String? =
        this[key]?.let { (it as? JsonPrimitive)?.content }

    fun registerPublisher(identity: PublisherIdentity) {
        publishers[identity.publisherId] = identity
    }

    fun clear() {
        publishers.clear()
    }

    override fun getPublisher(publisherId: String): PublisherIdentity? {
        return publishers[publisherId]
    }
}
