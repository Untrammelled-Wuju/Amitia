package com.amitia.amitia_app.runtime.proot

interface ProotComponent {
    fun availability(): ProotAvailability
    fun launch(request: ProotLaunchRequest, observer: ProotObserver): ProotSession
    fun close()
}