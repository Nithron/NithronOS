package com.nithron.sync.pinning

import java.security.cert.CertificateFactory
import java.security.cert.X509Certificate

/** Two self-signed certificates and their pins, as nosd computes them (Go's x509 RawSubjectPublicKeyInfo). */
object TestCerts {
    val nas: X509Certificate = load("nas-self-signed.pem")
    const val NAS_PIN = "01eba3af6196380131dca7babc106a40c2979866a6c163a4c7f939f2307cc35c"

    val other: X509Certificate = load("other-self-signed.pem")
    const val OTHER_PIN = "6cba41a7eeaf3f4cf5607b9e4a62b6ae0bc9119bc13490ce764ea365f54113d5"

    private fun load(name: String): X509Certificate {
        val stream = checkNotNull(TestCerts::class.java.classLoader).getResourceAsStream(name)
            ?: error("missing test resource $name")
        return stream.use { CertificateFactory.getInstance("X.509").generateCertificate(it) as X509Certificate }
    }
}
