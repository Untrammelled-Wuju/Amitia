package com.amitia.amitia_app.nativeprovider.accessibility

import android.accessibilityservice.AccessibilityService
import android.view.accessibility.AccessibilityEvent
import com.amitia.amitia_app.runtime.workflow.WorkflowDeviceEventIngress
import com.amitia.amitia_app.workflow.WorkflowTriggerCapabilityReporter
import org.json.JSONObject
import java.security.MessageDigest

class AmitiaAccessibilityService : AccessibilityService() {

    private val workflowIngress by lazy { WorkflowDeviceEventIngress(applicationContext) }

    override fun onServiceConnected() {
        super.onServiceConnected()
        ForegroundStateTracker.reset()
        AccessibilityServiceRegistry.attach(this)
        WorkflowTriggerCapabilityReporter.report(this)
    }

    override fun onAccessibilityEvent(event: AccessibilityEvent?) {
        val transition = event?.let(ForegroundStateTracker::update) ?: return
        val payload = JSONObject()
            .put("previousPackageName", transition.previousPackage)
            .put("packageName", transition.currentPackage)
            .put("activityName", transition.currentActivity ?: JSONObject.NULL)
            .put("changedAt", transition.changedAt)
            .put("generation", transition.generation)
        val digest = MessageDigest.getInstance("SHA-256")
            .digest("${transition.currentPackage}\n${transition.changedAt}".toByteArray(Charsets.UTF_8))
            .take(12)
            .joinToString("") { "%02x".format(it) }
        val eventID = "appfg:${transition.changedAt}:$digest"
        workflowIngress.emit("device.app.foreground", payload, "android.accessibility", eventID)
    }

    override fun onInterrupt() {
    }

    override fun onDestroy() {
        AccessibilityServiceRegistry.detach(this)
        ForegroundStateTracker.reset()
        WorkflowTriggerCapabilityReporter.report(this)
        super.onDestroy()
    }
}
