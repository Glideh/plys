package fr.plys.tv.data

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import java.io.IOException
import java.security.GeneralSecurityException
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

class SecureSessionStore(context: Context) : SessionStore {
    private val preferences = context.getSharedPreferences("session", Context.MODE_PRIVATE)
    private val alias = "plys.tv.refresh"

    override fun server(): String = preferences.getString("server", "") ?: ""

    override fun read(): StoredSession? = secure {
        val raw = preferences.getString("refresh", null) ?: return@secure null
        val parts = raw.split(":")
        if (parts.size != 2) throw ClientException(Failure.STORAGE)
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(128, Base64.decode(parts[0], Base64.NO_WRAP)))
        cipher.updateAAD(server().toByteArray(Charsets.UTF_8))
        StoredSession(server(), cipher.doFinal(Base64.decode(parts[1], Base64.NO_WRAP)).toString(Charsets.UTF_8))
    }

    override fun save(session: StoredSession) = secure {
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.ENCRYPT_MODE, key())
        cipher.updateAAD(session.server.toByteArray(Charsets.UTF_8))
        val encrypted = cipher.doFinal(session.refreshToken.toByteArray(Charsets.UTF_8))
        val raw = Base64.encodeToString(cipher.iv, Base64.NO_WRAP) + ":" +
            Base64.encodeToString(encrypted, Base64.NO_WRAP)
        if (!preferences.edit().putString("server", session.server).putString("refresh", raw).commit()) {
            throw ClientException(Failure.STORAGE)
        }
    }

    override fun clear() {
        if (!preferences.edit().remove("refresh").commit()) throw ClientException(Failure.STORAGE)
    }

    private fun key(): SecretKey {
        val keystore = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        val existing = keystore.getKey(alias, null)
        if (existing is SecretKey) return existing
        return KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore").apply {
            init(KeyGenParameterSpec.Builder(alias, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .build())
        }.generateKey()
    }

    private fun <T> secure(action: () -> T): T = try {
        action()
    } catch (error: GeneralSecurityException) {
        throw ClientException(Failure.STORAGE, cause = error)
    } catch (error: IOException) {
        throw ClientException(Failure.STORAGE, cause = error)
    } catch (error: IllegalArgumentException) {
        throw ClientException(Failure.STORAGE, cause = error)
    }
}
