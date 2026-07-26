package com.amitia.android.navigation

import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.asSharedFlow

@Singleton
class NavEventBus @Inject constructor() {

    private val _events = MutableSharedFlow<NavEvent>(extraBufferCapacity = 16)
    val events: SharedFlow<NavEvent> = _events.asSharedFlow()

    suspend fun emit(event: NavEvent) {
        _events.emit(event)
    }

    fun tryEmit(event: NavEvent): Boolean = _events.tryEmit(event)
}

sealed class NavEvent {
    data class OpenChat(val characterId: String, val conversationId: String? = null, val messageId: String? = null) : NavEvent()
    data class OpenCharacter(val characterId: String) : NavEvent()
    object OpenRuntime : NavEvent()
    object OpenHome : NavEvent()
    object ClearNotifications : NavEvent()
}
