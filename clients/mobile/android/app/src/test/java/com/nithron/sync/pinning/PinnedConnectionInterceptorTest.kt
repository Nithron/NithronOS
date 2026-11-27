package com.nithron.sync.pinning

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class PinnedConnectionInterceptorTest {
    private val pins = PinLookup { host -> if (host == "nas.local") TestCerts.NAS_PIN else null }

    @Test
    fun `a pooled connection to a pinned host is used only if it presented the pinned key`() {
        assertTrue(PinnedConnectionInterceptor.acceptable(pins, "nas.local", listOf(TestCerts.nas)))
        assertFalse(PinnedConnectionInterceptor.acceptable(pins, "nas.local", listOf(TestCerts.other)))
    }

    @Test
    fun `a pinned host needs TLS`() {
        assertFalse(PinnedConnectionInterceptor.acceptable(pins, "nas.local", null))
        assertFalse(PinnedConnectionInterceptor.acceptable(pins, "nas.local", emptyList()))
    }

    @Test
    fun `other hosts aren't affected`() {
        assertTrue(PinnedConnectionInterceptor.acceptable(pins, "example.com", null))
        assertTrue(PinnedConnectionInterceptor.acceptable(pins, "example.com", listOf(TestCerts.other)))
    }
}
