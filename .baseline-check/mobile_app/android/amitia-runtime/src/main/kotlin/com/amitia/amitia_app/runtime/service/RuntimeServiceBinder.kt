package com.amitia.amitia_app.runtime.service

import android.os.Binder

internal class RuntimeServiceBinder(
    internal val endpoint: RuntimeServiceEndpoint,
) : Binder()
