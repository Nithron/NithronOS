package com.nithron.sync.pinning

import java.security.cert.Certificate
import java.security.cert.X509Certificate
import javax.net.ssl.SSLPeerUnverifiedException
import javax.net.ssl.SSLSocket
import okhttp3.Interceptor
import okhttp3.Response

/**
 * Checks the pin on every request, not only on a new TLS handshake: OkHttp
 * reuses pooled connections without verifying them again, so a connection
 * opened before the pin changed (pairing again, with another key) would
 * otherwise still be used. A pinned host's request goes out only over a
 * connection whose server key is the current pin.
 */
class PinnedConnectionInterceptor(private val pins: PinLookup) : Interceptor {
    override fun intercept(chain: Interceptor.Chain): Response {
        val host = chain.request().url.host
        val connection = chain.connection()
        // The session's own certificates: OkHttp's Handshake.peerCertificates
        // are "cleaned" into a chain to a trusted CA, which a self-signed
        // certificate doesn't have, so reading them throws.
        val tls = connection?.socket() as? SSLSocket
        val peerCertificates = tls?.session?.let { runCatching { it.peerCertificates.toList() }.getOrNull() }
        if (!acceptable(pins, host, peerCertificates)) {
            // Never reused either.
            connection?.socket()?.close()
            throw SSLPeerUnverifiedException("$host's TLS key doesn't match the one this device was paired with")
        }
        return chain.proceed(chain.request())
    }

    companion object {
        /**
         * Whether a connection to host that presented peerCertificates (null:
         * no TLS) may carry a request: always for a host without a pin,
         * otherwise only if its leaf key is the pin.
         */
        fun acceptable(pins: PinLookup, host: String, peerCertificates: List<Certificate>?): Boolean {
            val pin = pins.pinFor(host) ?: return true
            val leaf = peerCertificates?.firstOrNull() as? X509Certificate ?: return false
            return KeyPin.matches(leaf, pin)
        }
    }
}
