package com.amitia.amitia_app.notifications.vendor;

import android.os.Bundle;
import com.amitia.amitia_app.notifications.VendorInboundBridge;
import com.huawei.hms.push.HmsMessageService;
import com.huawei.hms.push.RemoteMessage;

public final class AmitiaHuaweiMessageService extends HmsMessageService {
    @Override
    public void onNewToken(String token) {
        VendorInboundBridge.updateToken(getApplicationContext(), "hms", token);
    }

    @Override
    public void onNewToken(String token, Bundle bundle) {
        VendorInboundBridge.updateToken(getApplicationContext(), "hms", token);
    }

    @Override
    public void onMessageReceived(RemoteMessage message) {
        if (message == null) {
            return;
        }
        VendorInboundBridge.handleJson(
            getApplicationContext(),
            "hms",
            message.getData()
        );
    }
}
