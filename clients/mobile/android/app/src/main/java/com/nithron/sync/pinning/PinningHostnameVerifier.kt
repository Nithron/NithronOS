package com.nithron.sync.pinning

import java.security.cert.X509Certificate
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

/**
 * For a pinned host, the name in a self-signed certificate says nothing (it
 * is whatever nOS put there, often not the IP or .local name the phone
 * uses); the key is what identifies the machine. So a pinned host is
 * verified by its key again, and any other host by the platform's verifier.
 */
class PinningHostnameVerifier(
    private val pins: PinLookup,
    private val platform: HostnameVerifier,
) : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean {
        val pin = pins.pinFor(hostname) ?: return platform.verify(hostname, session)
        val leaf = runCatching { session.peerCertificates.firstOrNull() }.getOrNull() as? X509Certificate
        return leaf != null && KeyPin.matches(leaf, pin)
    }
}
