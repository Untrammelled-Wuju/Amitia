package com.amitia.amitia_app.notifications.vendor;

import android.content.Context;
import com.amitia.amitia_app.notifications.VendorInboundBridge;
import com.heytap.msp.push.mode.DataMessage;
import com.heytap.msp.push.service.DataMessageCallbackService;

/** Android Q+ OPPO/HeyTap pass-through entry point. */
public final class AmitiaOppoDataMessageService extends DataMessageCallbackService {
    @Override
    public void processMessage(Context context, DataMessage message) {
        if (context == null || message == null) {
            return;
        }
        super.processMessage(context, message);
        VendorInboundBridge.handleJson(context, "oppo", message.getContent());
    }
}
