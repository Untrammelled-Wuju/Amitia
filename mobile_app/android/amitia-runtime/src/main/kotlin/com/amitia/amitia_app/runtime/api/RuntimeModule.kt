package com.amitia.amitia_app.runtime.api

interface RuntimeModule {
    val controller: RuntimeController
    fun prootComponent(): Any
    fun close()
}
