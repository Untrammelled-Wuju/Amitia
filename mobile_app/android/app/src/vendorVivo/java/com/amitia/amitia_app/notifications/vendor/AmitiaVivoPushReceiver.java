package com.amitia.amitia_app.notifications.vendor;

import android.content.Context;
import com.amitia.amitia_app.notifications.VendorInboundBridge;
import com.vivo.push.model.UnvarnishedMessage;
import com.vivo.push.sdk.OpenClientPushMessageReceiver;
import java.util.Map;
import org.json.JSONObject;

/**
 * vivo transparent-message receiver.
 *
 * <p>This source set is compiled only when AMITIA_VIVO_NATIVE_DATA is
 * explicitly enabled and a vivo Push SDK is linked. The callback converges
 * into the same VendorInboundBridge/NotificationRenderer path used by the
 * other Android providers.</p>
 */
public final class AmitiaVivoPushReceiver extends OpenClientPushMessageReceiver {
    @Override
    public void onReceiveRegId(Context context, String regId) {
        if (context == null) {
            return;
        }
        VendorInboundBridge.updateToken(context, "vivo", regId);
    }

    @Override
    public void onTransmissionMessage(Context context, UnvarnishedMessage message) {
        super.onTransmissionMessage(context, message);
        if (context == null || message == null) {
            return;
        }
        VendorInboundBridge.handleJson(context, "vivo", extractPayload(message));
    }

    private static String extractPayload(UnvarnishedMessage message) {
        String direct = trim(message.getMessage());
        if (direct.startsWith("{")) {
            try {
                return new JSONObject(direct).toString();
            } catch (org.json.JSONException ignored) {
                // Fall back to structured custom parameters below.
            }
        }

        Map<String, String> params = message.getParams();
        if (params != null && !params.isEmpty()) {
            return new JSONObject(params).toString();
        }
        return "";
    }

    private static String trim(String value) {
        return value == null ? "" : value.trim();
    }
}
