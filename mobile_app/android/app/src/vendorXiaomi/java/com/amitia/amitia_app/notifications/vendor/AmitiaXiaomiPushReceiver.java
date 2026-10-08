package com.amitia.amitia_app.notifications.vendor;

import android.content.Context;
import com.amitia.amitia_app.notifications.VendorInboundBridge;
import com.xiaomi.mipush.sdk.ErrorCode;
import com.xiaomi.mipush.sdk.MiPushClient;
import com.xiaomi.mipush.sdk.MiPushCommandMessage;
import com.xiaomi.mipush.sdk.MiPushMessage;
import com.xiaomi.mipush.sdk.PushMessageReceiver;
import java.util.List;

public final class AmitiaXiaomiPushReceiver extends PushMessageReceiver {
    @Override
    public void onReceivePassThroughMessage(Context context, MiPushMessage message) {
        if (context == null || message == null) {
            return;
        }
        VendorInboundBridge.handleJson(context, "mipush", message.getContent());
    }

    @Override
    public void onReceiveRegisterResult(Context context, MiPushCommandMessage message) {
        if (context == null || message == null) {
            return;
        }
        if (!MiPushClient.COMMAND_REGISTER.equals(message.getCommand()) ||
            message.getResultCode() != ErrorCode.SUCCESS) {
            return;
        }
        List<String> arguments = message.getCommandArguments();
        if (arguments == null || arguments.isEmpty()) {
            return;
        }
        VendorInboundBridge.updateToken(context, "mipush", arguments.get(0));
    }
}
