package com.nithron.sync.pinning

import java.security.MessageDigest
import java.security.cert.X509Certificate

/**
 * A server key pin: SHA-256 of the certificate's SubjectPublicKeyInfo, as
 * lowercase hex. The same value nosd reports as server_key_sha256 and the
 * pairing link carries as key= (clients/sync-core/api/tls.go).
 */
object KeyPin {
    private const val SHA256_HEX_LENGTH = 64
    private val HEX = Regex("^[0-9a-f]+$")

    /** The pin in canonical form (no colons or spaces, lowercase), or null if it isn't one. */
    fun normalize(pin: String): String? {
        val p = pin.trim().replace(":", "").replace(" ", "").lowercase()
        return if (p.length == SHA256_HEX_LENGTH && HEX.matches(p)) p else null
    }

    /** The pin of a certificate. */
    fun of(cert: X509Certificate): String =
        MessageDigest.getInstance("SHA-256")
            .digest(cert.publicKey.encoded)
            .joinToString("") { "%02x".format(it) }

    /** Whether cert's key is the pinned one (compared in constant time). */
    fun matches(cert: X509Certificate, pin: String): Boolean {
        val expected = normalize(pin) ?: return false
        return MessageDigest.isEqual(of(cert).toByteArray(), expected.toByteArray())
    }
}
