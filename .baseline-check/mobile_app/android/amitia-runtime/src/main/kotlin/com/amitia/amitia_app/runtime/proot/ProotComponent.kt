package com.amitia.amitia_app.runtime.proot

interface ProotComponent {
    fun availability(): ProotAvailability
    fun launch(request: ProotLaunchRequest, observer: ProotObserver, generation: Long): ProotSession
    fun launchProbe(request: ProotLaunchRequest, observer: ProotObserver, generation: Long): ProotSession
    fun currentSession(): ProotSession?
    fun stop(): ProotStopResult
    fun close()
}
