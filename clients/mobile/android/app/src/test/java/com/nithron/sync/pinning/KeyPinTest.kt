package com.nithron.sync.pinning

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class KeyPinTest {
    @Test
    fun `a certificate's pin is the SHA-256 of its public key, as nosd reports it`() {
        assertEquals(TestCerts.NAS_PIN, KeyPin.of(TestCerts.nas))
        assertEquals(TestCerts.OTHER_PIN, KeyPin.of(TestCerts.other))
    }

    @Test
    fun `matches only the certificate with that key`() {
        assertTrue(KeyPin.matches(TestCerts.nas, TestCerts.NAS_PIN))
        assertFalse(KeyPin.matches(TestCerts.other, TestCerts.NAS_PIN))
    }

    @Test
    fun `a pin may have colons, spaces and capitals`() {
        val colons = TestCerts.NAS_PIN.uppercase().chunked(2).joinToString(":")

        assertEquals(TestCerts.NAS_PIN, KeyPin.normalize(" $colons "))
        assertTrue(KeyPin.matches(TestCerts.nas, colons))
    }

    @Test
    fun `refuses what isn't a SHA-256 in hex`() {
        assertNull(KeyPin.normalize("short"))
        assertNull(KeyPin.normalize("zz".repeat(32)))
        assertNull(KeyPin.normalize(TestCerts.NAS_PIN + "00"))
        assertFalse(KeyPin.matches(TestCerts.nas, "not a pin"))
    }
}
