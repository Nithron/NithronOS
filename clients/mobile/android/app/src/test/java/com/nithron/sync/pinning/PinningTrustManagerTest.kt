package com.nithron.sync.pinning

import java.security.cert.CertificateException
import java.security.cert.X509Certificate
import javax.net.ssl.X509TrustManager
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class PinningTrustManagerTest {
    /** The platform's CA check: refuses every self-signed chain, and counts its calls. */
    private class RefusingPlatform : X509TrustManager {
        var calls = 0

        override fun checkServerTrusted(chain: Array<X509Certificate>, authType: String) {
            calls++
            throw CertificateException("no CA vouches for it")
        }

        override fun checkClientTrusted(chain: Array<X509Certificate>, authType: String) = Unit

        override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
    }

    private val platform = RefusingPlatform()
    private val pins = PinLookup { host -> if (host == "nas.local") TestCerts.NAS_PIN else null }
    private val trust = PinningTrustManager(pins, platform)

    private fun check(host: String?, vararg chain: X509Certificate) =
        trust.checkServerTrustedFor(arrayOf(*chain), host) { platform.checkServerTrusted(arrayOf(*chain), "ECDHE_ECDSA") }

    @Test
    fun `a pinned host's self-signed certificate is trusted by its key, without the CA check`() {
        check("nas.local", TestCerts.nas)

        assertEquals(0, platform.calls)
    }

    @Test
    fun `a pinned host presenting another key is refused`() {
        val e = assertThrows(CertificateException::class.java) { check("nas.local", TestCerts.other) }

        assertTrue(e.message!!.contains("doesn't match"))
        assertEquals(0, platform.calls)
    }

    @Test
    fun `only the leaf counts, the pinned key further up the chain isn't enough`() {
        assertThrows(CertificateException::class.java) { check("nas.local", TestCerts.other, TestCerts.nas) }
    }

    @Test
    fun `a pinned host with no certificate is refused`() {
        assertThrows(CertificateException::class.java) { check("nas.local") }
    }

    @Test
    fun `a host without a pin gets the platform's validation`() {
        assertThrows(CertificateException::class.java) { check("example.com", TestCerts.nas) }

        assertEquals(1, platform.calls)
    }

    @Test
    fun `without a host name the platform decides`() {
        assertThrows(CertificateException::class.java) { check(null, TestCerts.nas) }
        assertThrows(CertificateException::class.java) {
            trust.checkServerTrusted(arrayOf(TestCerts.nas), "ECDHE_ECDSA")
        }

        assertEquals(2, platform.calls)
    }
}
