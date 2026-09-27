package com.amitia.amitia_app.accessibility;

interface IAmitiaAccessibilityProvider {
    String status();
    String snapshot(String payloadJson);
    String performNodeAction(String payloadJson);
    String performClick(int x, int y);
    String performLongPress(int x, int y, long durationMs);
    String performSwipe(int startX, int startY, int endX, int endY, long durationMs);
    String performGlobalAction(int actionId);
}
