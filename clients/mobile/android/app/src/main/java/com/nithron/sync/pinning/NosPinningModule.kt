package com.nithron.sync.pinning

import com.facebook.react.bridge.Promise
import com.facebook.react.bridge.ReactApplicationContext
import com.facebook.react.bridge.ReactContextBaseJavaModule
import com.facebook.react.bridge.ReactMethod

/**
 * NativeModules.NosPinning: lets JavaScript pin a server's key after
 * pairing (src/native/pinning.ts). The pins are kept natively, so the HTTP
 * client has them from the app's first request.
 */
class NosPinningModule(
    reactContext: ReactApplicationContext,
) : ReactContextBaseJavaModule(reactContext) {
    private val store = PinStore.get(reactContext)

    override fun getName(): String = NAME

    @ReactMethod
    fun setPin(host: String, pinHex: String, promise: Promise) {
        try {
            if (store.set(host, pinHex)) {
                promise.resolve(null)
            } else {
                promise.reject("E_PIN_INVALID", "the server key must be a SHA-256 in hex (64 characters)")
            }
        } catch (e: IllegalStateException) {
            promise.reject("E_PIN_STORE", e.message, e)
        }
    }

    @ReactMethod
    fun clearPin(host: String, promise: Promise) {
        try {
            store.clear(host)
            promise.resolve(null)
        } catch (e: IllegalStateException) {
            promise.reject("E_PIN_STORE", e.message, e)
        }
    }

    @ReactMethod
    fun clearAll(promise: Promise) {
        try {
            store.clearAll()
            promise.resolve(null)
        } catch (e: IllegalStateException) {
            promise.reject("E_PIN_STORE", e.message, e)
        }
    }

    companion object {
        const val NAME = "NosPinning"
    }
}
