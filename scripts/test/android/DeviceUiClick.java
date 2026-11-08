import android.os.Bundle;

import com.android.uiautomator.core.UiObject;
import com.android.uiautomator.core.UiSelector;
import com.android.uiautomator.testrunner.UiAutomatorTestCase;

public class DeviceUiClick extends UiAutomatorTestCase {
    public void testClickByDescription() throws Exception {
        Bundle params = getParams();
        String description = params.getString("description", "").trim();
        if (description.isEmpty()) {
            throw new IllegalArgumentException("description is required");
        }
        UiObject target = new UiObject(new UiSelector().description(description));
        assertTrue("target not found: " + description, target.waitForExists(5000));
        assertTrue("target not clickable: " + description, target.click());
    }
}
