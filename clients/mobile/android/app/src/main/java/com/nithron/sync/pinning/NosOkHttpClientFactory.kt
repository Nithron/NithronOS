package com.nithron.sync.pinning

import android.content.Context
import com.facebook.react.modules.network.OkHttpClientFactory
import com.facebook.react.modules.network.OkHttpClientProvider
import java.lang.ref.WeakReference
import java.security.KeyStore
import javax.net.ssl.SSLContext
import javax.net.ssl.TrustManagerFactory
import javax.net.ssl.X509TrustManager
import okhttp3.OkHttpClient

/**
 * The HTTP client React Native's fetch uses, as React Native builds it
 * (cookie jar, cache), with the pinned-key trust check added and redirects
 * not followed.
 */
class NosOkHttpClientFactory(
    private val context: Context,
    private val pins: PinStore,
) : OkHttpClientFactory {
    override fun createNewNetworkModuleClient(): OkHttpClient {
        val trustManager = PinningTrustManager(pins, platformTrustManager())
        val sslContext = SSLContext.getInstance("TLS").apply {
            init(null, arrayOf(trustManager), null)
        }
        val reactNativeDefaults = OkHttpClientProvider.createClientBuilder(context).build()
        return reactNativeDefaults.newBuilder()
            .sslSocketFactory(sslContext.socketFactory, trustManager)
            // OkHttp's own verifier for every host without a pin.
            .hostnameVerifier(PinningHostnameVerifier(pins, reactNativeDefaults.hostnameVerifier))
            // The app only talks to the paired nOS machine, which doesn't
            // redirect; following one could carry a request body (the
            // refresh token) to another host.
            .followRedirects(false)
            .followSslRedirects(false)
            .addNetworkInterceptor(PinnedConnectionInterceptor(pins))
            .build()
            .also(::dropConnectionsWhenPinsChange)
    }

    /** New pins take effect at once: idle connections made under the old ones go. */
    private fun dropConnectionsWhenPinsChange(client: OkHttpClient) {
        val ref = WeakReference(client)
        pins.addChangeListener { ref.get()?.connectionPool?.evictAll() }
    }

    private fun platformTrustManager(): X509TrustManager {
        val factory = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm())
        factory.init(null as KeyStore?)
        return factory.trustManagers.filterIsInstance<X509TrustManager>().first()
    }
}
