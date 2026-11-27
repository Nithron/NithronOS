package com.nithron.sync.pinning

import android.content.SharedPreferences
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class PinStoreTest {
    /** SharedPreferences in memory (the JVM tests have no Android runtime). */
    private class MemoryPrefs : SharedPreferences {
        val values = mutableMapOf<String, Any?>()

        override fun getAll(): Map<String, *> = values.toMap()
        override fun getString(key: String, defValue: String?): String? = values[key] as? String ?: defValue
        override fun getStringSet(key: String, defValues: Set<String>?): Set<String>? = defValues
        override fun getInt(key: String, defValue: Int) = defValue
        override fun getLong(key: String, defValue: Long) = defValue
        override fun getFloat(key: String, defValue: Float) = defValue
        override fun getBoolean(key: String, defValue: Boolean) = defValue
        override fun contains(key: String) = values.containsKey(key)
        override fun registerOnSharedPreferenceChangeListener(l: SharedPreferences.OnSharedPreferenceChangeListener) = Unit
        override fun unregisterOnSharedPreferenceChangeListener(l: SharedPreferences.OnSharedPreferenceChangeListener) = Unit

        override fun edit(): SharedPreferences.Editor = object : SharedPreferences.Editor {
            private val changes = mutableMapOf<String, Any?>()
            private var clear = false

            override fun putString(key: String, value: String?) = apply { changes[key] = value }
            override fun putStringSet(key: String, values: Set<String>?) = apply { changes[key] = values }
            override fun putInt(key: String, value: Int) = apply { changes[key] = value }
            override fun putLong(key: String, value: Long) = apply { changes[key] = value }
            override fun putFloat(key: String, value: Float) = apply { changes[key] = value }
            override fun putBoolean(key: String, value: Boolean) = apply { changes[key] = value }
            override fun remove(key: String) = apply { changes[key] = null }
            override fun clear() = apply { clear = true }
            override fun apply() {
                commit()
            }

            override fun commit(): Boolean {
                if (clear) values.clear()
                changes.forEach { (k, v) -> if (v == null) values.remove(k) else values[k] = v }
                return true
            }
        }
    }

    private val prefs = MemoryPrefs()

    @Test
    fun `keeps a normalized pin per host, case-insensitively`() {
        val store = PinStore(prefs)

        assertTrue(store.set("NAS.local", TestCerts.NAS_PIN.uppercase()))

        assertEquals(TestCerts.NAS_PIN, store.pinFor("nas.local"))
        assertEquals(TestCerts.NAS_PIN, store.pinFor("Nas.Local"))
        assertNull(store.pinFor("other.local"))
    }

    @Test
    fun `refuses a pin that isn't one`() {
        val store = PinStore(prefs)

        assertFalse(store.set("nas.local", "nope"))
        assertNull(store.pinFor("nas.local"))
    }

    @Test
    fun `the pins outlive the store (an app restart)`() {
        PinStore(prefs).set("[fe80::1]", TestCerts.NAS_PIN)

        assertEquals(TestCerts.NAS_PIN, PinStore(prefs).pinFor("fe80::1"))
    }

    @Test
    fun `tells listeners about every change`() {
        val store = PinStore(prefs)
        var changes = 0
        store.addChangeListener { changes++ }

        store.set("a", TestCerts.NAS_PIN)
        store.set("a", "not a pin")
        store.clear("a")
        store.clearAll()

        assertEquals(3, changes)
    }

    @Test
    fun `clears one pin, or all`() {
        val store = PinStore(prefs)
        store.set("a", TestCerts.NAS_PIN)
        store.set("b", TestCerts.OTHER_PIN)

        store.clear("a")
        assertNull(store.pinFor("a"))
        assertEquals(TestCerts.OTHER_PIN, store.pinFor("b"))

        store.clearAll()
        assertNull(PinStore(prefs).pinFor("b"))
    }
}
