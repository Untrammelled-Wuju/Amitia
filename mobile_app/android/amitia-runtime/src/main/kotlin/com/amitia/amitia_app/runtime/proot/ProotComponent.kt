package com.amitia.amitia_app.runtime.proot

interface ProotComponent {
    fun availability(): ProotAvailability
    fun launch(request: ProotLaunchRequest, observer: ProotObserver): ProotSession
    fun launchProbe(request: ProotLaunchRequest, observer: ProotObserver): ProotSession
    fun currentSession(): ProotSession?
    fun stop(): ProotStopResult
    fun close()
}