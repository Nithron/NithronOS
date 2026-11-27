package com.nithron.sync.pinning

import java.net.Socket
import java.security.cert.CertificateException
import java.security.cert.X509Certificate
import javax.net.ssl.SSLEngine
import javax.net.ssl.SSLSocket
import javax.net.ssl.X509ExtendedTrustManager
import javax.net.ssl.X509TrustManager

/**
 * Trusts a pinned host's server by its key alone, and every other host the
 * platform's way.
 *
 * nOS's web certificate is self-signed, so no certificate authority vouches
 * for it; the pairing link carries the SHA-256 of its public key instead.
 * For a host with a pin, the chain isn't checked against CAs (it can't be):
 * the connection is accepted if and only if the leaf certificate's key is
 * the pinned one, which is stricter for a single known machine. A host
 * without a pin gets the platform's normal validation.
 */
class PinningTrustManager(
    private val pins: PinLookup,
    private val platform: X509TrustManager,
) : X509ExtendedTrustManager() {

    override fun checkServerTrusted(chain: Array<X509Certificate>, authType: String, socket: Socket?) {
        val host = (socket as? SSLSocket)?.handshakeSession?.peerHost
        checkServerTrustedFor(chain, host) {
            if (platform is X509ExtendedTrustManager) {
                platform.checkServerTrusted(chain, authType, socket)
            } else {
                platform.checkServerTrusted(chain, authType)
            }
        }
    }

    override fun checkServerTrusted(chain: Array<X509Certificate>, authType: String, engine: SSLEngine?) {
        val host = engine?.handshakeSession?.peerHost ?: engine?.peerHost
        checkServerTrustedFor(chain, host) {
            if (platform is X509ExtendedTrustManager) {
                platform.checkServerTrusted(chain, authType, engine)
            } else {
                platform.checkServerTrusted(chain, authType)
            }
        }
    }

    /** Without a host there is no pin to look up: the platform decides. */
    override fun checkServerTrusted(chain: Array<X509Certificate>, authType: String) {
        platform.checkServerTrusted(chain, authType)
    }

    /**
     * The decision itself: host's pin when it has one, otherwise
     * platformCheck (which throws when the platform refuses the chain).
     */
    internal fun checkServerTrustedFor(
        chain: Array<X509Certificate>,
        host: String?,
        platformCheck: () -> Unit,
    ) {
        val pin = host?.let(pins::pinFor)
        if (pin == null) {
            platformCheck()
            return
        }
        val leaf = chain.firstOrNull() ?: throw CertificateException("$host sent no certificate")
        if (!KeyPin.matches(leaf, pin)) {
            throw CertificateException(
                "$host's TLS key doesn't match the one this device was paired with",
            )
        }
    }

    override fun checkClientTrusted(chain: Array<X509Certificate>, authType: String, socket: Socket?) {
        platform.checkClientTrusted(chain, authType)
    }

    override fun checkClientTrusted(chain: Array<X509Certificate>, authType: String, engine: SSLEngine?) {
        platform.checkClientTrusted(chain, authType)
    }

    override fun checkClientTrusted(chain: Array<X509Certificate>, authType: String) {
        platform.checkClientTrusted(chain, authType)
    }

    override fun getAcceptedIssuers(): Array<X509Certificate> = platform.acceptedIssuers
}
