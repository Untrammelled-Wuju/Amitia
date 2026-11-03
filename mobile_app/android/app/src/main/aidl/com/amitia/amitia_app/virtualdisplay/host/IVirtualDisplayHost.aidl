package com.amitia.amitia_app.virtualdisplay.host;

import android.view.Surface;

interface IVirtualDisplayHost {
    String execute(String requestJson);
    String bindSurface(int displayId, in Surface surface);
    String detachSurface(int displayId);
    void shutdown();
}
