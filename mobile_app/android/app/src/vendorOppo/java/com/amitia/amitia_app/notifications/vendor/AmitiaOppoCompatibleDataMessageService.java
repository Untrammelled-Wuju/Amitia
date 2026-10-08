package com.amitia.amitia_app.notifications.vendor;

import android.content.Context;
import com.amitia.amitia_app.notifications.VendorInboundBridge;
import com.heytap.msp.push.mode.DataMessage;
import com.heytap.msp.push.service.CompatibleDataMessageCallbackService;

/** Pre-Android-Q OPPO/HeyTap pass-through entry point. */
public final class AmitiaOppoCompatibleDataMessageService extends CompatibleDataMessageCallbackService {
    @Override
    public void processMessage(Context context, DataMessage message) {
        if (context == null || message == null) {
            return;
        }
        super.processMessage(context.getApplicationContext(), message);
        VendorInboundBridge.handleJson(context, "oppo", message.getContent());
    }
}
