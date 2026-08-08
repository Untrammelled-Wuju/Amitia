package com.amitia.amitia_app.runtime.service

internal object RuntimeServiceContract {
    const val ACTION_START_HOST = "com.amitia.amitia_app.runtime.action.START_HOST"
    const val ACTION_STOP_HOST = "com.amitia.amitia_app.runtime.action.STOP_HOST"

    const val EXTRA_REQUEST_ID = "com.amitia.amitia_app.runtime.extra.REQUEST_ID"

    const val NOTIFICATION_CHANNEL_ID = "runtime_service"
    const val NOTIFICATION_ID = 0x52435541

    const val FOREGROUND_SERVICE_TYPE = android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE

    const val SPECIAL_USE_FGS_SUBTYPE = "amitia_embedded_runtime"
}
