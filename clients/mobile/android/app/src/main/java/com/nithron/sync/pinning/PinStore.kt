package com.nithron.sync.pinning

import android.content.Context
import android.content.SharedPreferences
import java.util.concurrent.CopyOnWriteArrayList

/** Which key a host must present; null means "check it the normal way". */
fun interface PinLookup {
    fun pinFor(host: String): String?
}

/**
 * The pinned server keys, by host name (lowercase). Kept in
 * SharedPreferences so the HTTP client has them before any JavaScript runs.
 */
class PinStore internal constructor(private val prefs: SharedPreferences) : PinLookup {
    @Volatile
    private var pins: Map<String, String> = load()
    private val listeners = CopyOnWriteArrayList<() -> Unit>()

    /** Runs listener after every change of the pins. */
    fun addChangeListener(listener: () -> Unit) {
        listeners.add(listener)
    }

    private fun changed() = listeners.forEach { it() }

    override fun pinFor(host: String): String? = pins[hostKey(host)]

    /** Pins host to pin; false if pin isn't a SHA-256 in hex. */
    @Synchronized
    fun set(host: String, pin: String): Boolean {
        val normalized = KeyPin.normalize(pin) ?: return false
        check(prefs.edit().putString(hostKey(host), normalized).commit()) { "the pin couldn't be saved" }
        pins = pins + (hostKey(host) to normalized)
        changed()
        return true
    }

    @Synchronized
    fun clear(host: String) {
        check(prefs.edit().remove(hostKey(host)).commit()) { "the pin couldn't be removed" }
        pins = pins - hostKey(host)
        changed()
    }

    @Synchronized
    fun clearAll() {
        check(prefs.edit().clear().commit()) { "the pins couldn't be removed" }
        pins = emptyMap()
        changed()
    }

    private fun load(): Map<String, String> =
        prefs.all.mapNotNull { (host, pin) ->
            (pin as? String)?.let(KeyPin::normalize)?.let { host to it }
        }.toMap()

    companion object {
        private const val PREFS_NAME = "nos_server_pins"

        @Volatile
        private var instance: PinStore? = null

        fun get(context: Context): PinStore =
            instance ?: synchronized(this) {
                instance ?: PinStore(
                    context.applicationContext.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE),
                ).also { instance = it }
            }

        /** Host names compare without case, and an IPv6 literal without its brackets. */
        fun hostKey(host: String): String = host.trim().removePrefix("[").removeSuffix("]").lowercase()
    }
}
