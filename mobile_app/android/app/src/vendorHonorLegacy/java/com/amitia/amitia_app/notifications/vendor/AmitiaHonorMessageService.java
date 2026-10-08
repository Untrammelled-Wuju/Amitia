package com.amitia.amitia_app.notifications.vendor;

import com.amitia.amitia_app.notifications.VendorInboundBridge;
import com.honor.push.sdk.HonorMessageService;
import com.honor.push.sdk.HonorPushDataMsg;
import java.lang.reflect.Method;

public final class AmitiaHonorMessageService extends HonorMessageService {
    @Override
    public void onNewToken(String pushToken) {
        VendorInboundBridge.updateToken(getApplicationContext(), "honor", pushToken);
    }

    @Override
    public void onMessageReceived(HonorPushDataMsg message) {
        if (message == null) {
            return;
        }
        VendorInboundBridge.handleJson(
            getApplicationContext(),
            "honor",
            extractPayload(message)
        );
    }

    private String extractPayload(HonorPushDataMsg message) {
        for (String methodName : new String[]{"getData", "getMessage", "getDataMsg"}) {
            try {
                Method method = message.getClass().getMethod(methodName);
                Object value = method.invoke(message);
                if (value != null && !value.toString().trim().isEmpty()) {
                    return value.toString();
                }
            } catch (ReflectiveOperationException ignored) {
            }
        }
        return "";
    }
}
