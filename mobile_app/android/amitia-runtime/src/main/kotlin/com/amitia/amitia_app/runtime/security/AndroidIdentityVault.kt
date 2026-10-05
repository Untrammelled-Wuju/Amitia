package com.amitia.amitia_app.runtime.security

import android.content.Context
import android.os.Build
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import java.io.File
import java.io.FileOutputStream
import java.security.KeyStore
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

internal class AndroidIdentityVault(context: Context) {
    private val path = File(context.noBackupFilesDir, "amitia-device-storage-key")
    private val alias = "amitia.device.storage.v1"

    fun key(): String {
        synchronized(lock) {
        require(Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) { "设备不支持所需的安全存储" }
        val store = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        val existing = store.getKey(alias, null) as? SecretKey
        if (path.exists() && existing == null) error("设备安全密钥已丢失，拒绝读取复制的身份")
        val master = existing ?: KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore").run {
            init(KeyGenParameterSpec.Builder(alias, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setKeySize(256)
                .build())
            generateKey()
        }
        if (path.exists()) {
            val encoded = path.readBytes()
            require(encoded.size >= 12 + 16) { "设备安全存储数据无效" }
            val cipher = Cipher.getInstance("AES/GCM/NoPadding")
            cipher.init(Cipher.DECRYPT_MODE, master, GCMParameterSpec(128, encoded.copyOfRange(0, 12)))
            cipher.updateAAD(alias.toByteArray(Charsets.UTF_8))
            val plaintext = cipher.doFinal(encoded.copyOfRange(12, encoded.size))
            require(plaintext.size == 32) { "设备安全密钥无效" }
            return Base64.encodeToString(plaintext, Base64.NO_WRAP)
        }
        val plaintext = ByteArray(32).also { SecureRandom().nextBytes(it) }
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.ENCRYPT_MODE, master)
        cipher.updateAAD(alias.toByteArray(Charsets.UTF_8))
        require(cipher.iv.size == 12) { "设备安全随机数无效" }
        path.parentFile?.mkdirs()
        val temporary = File(path.parentFile, "${path.name}.tmp")
        FileOutputStream(temporary).use { output -> output.write(cipher.iv + cipher.doFinal(plaintext)); output.flush(); output.fd.sync() }
        check(temporary.renameTo(path)) { "无法保存设备安全密钥" }
        return Base64.encodeToString(plaintext, Base64.NO_WRAP)
        }
    }

    companion object {
        private val lock = Any()
    }
}
