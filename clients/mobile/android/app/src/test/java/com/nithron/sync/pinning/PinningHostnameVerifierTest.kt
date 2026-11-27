package com.nithron.sync.pinning

import java.lang.reflect.Proxy
import java.security.cert.Certificate
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLPeerUnverifiedException
import javax.net.ssl.SSLSession
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class PinningHostnameVerifierTest {
    private var platformCalls = 0
    private val platform = HostnameVerifier { _, _ ->
        platformCalls++
        false
    }
    private val pins = PinLookup { host -> if (host == "192.168.1.10") TestCerts.NAS_PIN else null }
    private val verifier = PinningHostnameVerifier(pins, platform)

    /** An SSLSession whose peer sent certs (null: peer not verified). */
    private fun session(vararg certs: Certificate?): SSLSession =
        Proxy.newProxyInstance(javaClass.classLoader, arrayOf(SSLSession::class.java)) { _, method, _ ->
            when (method.name) {
                "getPeerCertificates" ->
                    if (certs.isEmpty()) throw SSLPeerUnverifiedException("none") else arrayOf(*certs)
                else -> null
            }
        } as SSLSession

    @Test
    fun `a pinned host matches by key, whatever name the certificate has`() {
        assertTrue(verifier.verify("192.168.1.10", session(TestCerts.nas)))
        assertEquals(0, platformCalls)
    }

    @Test
    fun `a pinned host with another key fails`() {
        assertFalse(verifier.verify("192.168.1.10", session(TestCerts.other)))
        assertFalse(verifier.verify("192.168.1.10", session()))
    }

    @Test
    fun `other hosts go to the platform's verifier`() {
        assertFalse(verifier.verify("example.com", session(TestCerts.nas)))
        assertEquals(1, platformCalls)
    }
}
