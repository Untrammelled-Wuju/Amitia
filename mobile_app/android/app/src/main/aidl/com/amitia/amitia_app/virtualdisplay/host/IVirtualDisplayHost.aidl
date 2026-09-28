package com.amitia.amitia_app.virtualdisplay.host;

interface IVirtualDisplayHost {
    String execute(String requestJson);
    void shutdown();
}
