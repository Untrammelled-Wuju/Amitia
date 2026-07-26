package com.amitia.core.security

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import androidx.core.content.edit
import com.amitia.core.logging.Logger
import dagger.hilt.android.qualifiers.ApplicationContext
import java.security.KeyStore
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

@Singleton
class KeystoreManager @Inject constructor(
    @ApplicationContext private val context: Context,
    private val logger: Logger
) {

    private val sharedPrefs by lazy {
        context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
    }

    private val keyStore by lazy {
        KeyStore.getInstance(KEYSTORE_TYPE).apply { load(null) }
    }

    suspend fun saveToken(alias: String, token: String): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            if (token.isEmpty()) {
                deleteToken(alias)
                return@runCatching
            }
            val key = ensureKey(alias)
            val cipher = Cipher.getInstance(TRANSFORMATION)
            cipher.init(Cipher.ENCRYPT_MODE, key)
            val encrypted = cipher.doFinal(token.toByteArray(Charsets.UTF_8))
            val iv = cipher.iv
            val combined = iv + encrypted
            val encoded = android.util.Base64.encodeToString(combined, android.util.Base64.NO_WRAP)
            sharedPrefs.edit { putString(prefsKey(alias), encoded) }
            logger.d(TAG, "token saved for alias=${redactAlias(alias)}")
        }.onFailure { t ->
            logger.e(TAG, "saveToken failed for alias=${redactAlias(alias)}", t)
        }
    }

    suspend fun loadToken(alias: String): String? = withContext(Dispatchers.IO) {
        runCatching {
            val encoded = sharedPrefs.getString(prefsKey(alias), null) ?: return@runCatching null
            val key = ensureKey(alias)
            val combined = android.util.Base64.decode(encoded, android.util.Base64.NO_WRAP)
            if (combined.size < IV_LENGTH) return@runCatching null
            val iv = combined.copyOfRange(0, IV_LENGTH)
            val encrypted = combined.copyOfRange(IV_LENGTH, combined.size)
            val cipher = Cipher.getInstance(TRANSFORMATION)
            val spec = GCMParameterSpec(GCM_TAG_LENGTH_BITS, iv)
            cipher.init(Cipher.DECRYPT_MODE, key, spec)
            val decrypted = cipher.doFinal(encrypted)
            String(decrypted, Charsets.UTF_8)
        }.onFailure { t ->
            logger.w(TAG, "loadToken failed for alias=${redactAlias(alias)}", t)
        }.getOrNull()
    }

    suspend fun deleteToken(alias: String) = withContext(Dispatchers.IO) {
        runCatching {
            sharedPrefs.edit { remove(prefsKey(alias)) }
            runCatching { keyStore.deleteEntry(alias) }
            logger.d(TAG, "token deleted for alias=${redactAlias(alias)}")
        }
    }

    fun generateLocalAuthToken(): String {
        val bytes = ByteArray(TOKEN_BYTES)
        SecureRandom().nextBytes(bytes)
        return bytes.joinToString("") { "%02x".format(it) }
    }

    fun saveSecureString(key: String, value: String) {
        if (value.isEmpty()) {
            sharedPrefs.edit { remove(key) }
            return
        }
        val alias = "amitia_secure_$key"
        val cipher = Cipher.getInstance(TRANSFORMATION)
        cipher.init(Cipher.ENCRYPT_MODE, ensureKey(alias))
        val encrypted = cipher.doFinal(value.toByteArray(Charsets.UTF_8))
        val iv = cipher.iv
        val combined = iv + encrypted
        val encoded = android.util.Base64.encodeToString(combined, android.util.Base64.NO_WRAP)
        sharedPrefs.edit { putString(key, encoded) }
    }

    fun loadSecureString(key: String): String? {
        val encoded = sharedPrefs.getString(key, null) ?: return null
        return runCatching {
            val alias = "amitia_secure_$key"
            val combined = android.util.Base64.decode(encoded, android.util.Base64.NO_WRAP)
            if (combined.size < IV_LENGTH) return null
            val iv = combined.copyOfRange(0, IV_LENGTH)
            val encrypted = combined.copyOfRange(IV_LENGTH, combined.size)
            val cipher = Cipher.getInstance(TRANSFORMATION)
            val spec = GCMParameterSpec(GCM_TAG_LENGTH_BITS, iv)
            cipher.init(Cipher.DECRYPT_MODE, ensureKey(alias), spec)
            String(cipher.doFinal(encrypted), Charsets.UTF_8)
        }.getOrNull()
    }

    fun removeSecureString(key: String) {
        sharedPrefs.edit { remove(key) }
        runCatching { keyStore.deleteEntry("amitia_secure_$key") }
    }

    private fun ensureKey(alias: String): SecretKey {
        val existing = keyStore.getKey(alias, null) as? SecretKey
        if (existing != null) return existing
        val generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, KEYSTORE_TYPE)
        val spec = KeyGenParameterSpec.Builder(
            alias,
            KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT
        )
            .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
            .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
            .setKeySize(256)
            .build()
        generator.init(spec)
        return generator.generateKey()
    }

    private fun prefsKey(alias: String): String = "token_v1_$alias"

    private fun redactAlias(alias: String): String {
        if (alias.length <= 4) return "***"
        return alias.take(2) + "***" + alias.takeLast(2)
    }

    companion object {
        const val ALIAS_SESSION_TOKEN = "amitia_session_token"
        const val ALIAS_LOCAL_AUTH_TOKEN = "amitia_local_auth_token"
        const val ALIAS_REMOTE_TOKEN = "amitia_remote_token"

        private const val TAG = "KeystoreManager"
        private const val PREFS_NAME = "amitia_keystore"
        private const val KEYSTORE_TYPE = "AndroidKeyStore"
        private const val TRANSFORMATION = "AES/GCM/NoPadding"
        private const val IV_LENGTH = 12
        private const val GCM_TAG_LENGTH_BITS = 128
        private const val TOKEN_BYTES = 32
    }
}
