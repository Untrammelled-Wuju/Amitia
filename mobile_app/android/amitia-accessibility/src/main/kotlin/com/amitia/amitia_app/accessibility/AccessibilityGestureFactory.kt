package com.amitia.amitia_app.accessibility

import android.accessibilityservice.GestureDescription
import android.graphics.Path
import android.os.Build
import org.json.JSONArray
import org.json.JSONObject

object AccessibilityGestureFactory {

    data class Result(
        val gesture: GestureDescription? = null,
        val message: String = "",
    )

    private const val MAX_STROKES = 16
    private const val MAX_POINTS_PER_STROKE = 512
    private const val MAX_DURATION_MS = 60000L
    private const val MAX_START_TIME_MS = 60000L

    fun parse(payload: JSONObject): Result {
        val strokes = payload.optJSONArray("strokes") ?: return Result(message = "strokes is required")
        if (strokes.length() == 0 || strokes.length() > MAX_STROKES) {
            return Result(message = "strokes must contain between 1 and $MAX_STROKES entries")
        }
        val builder = GestureDescription.Builder()
        for (index in 0 until strokes.length()) {
            val strokeObject = strokes.optJSONObject(index)
                ?: return Result(message = "stroke $index must be an object")
            val points = strokeObject.optJSONArray("points")
                ?: return Result(message = "stroke $index points is required")
            if (points.length() == 0 || points.length() > MAX_POINTS_PER_STROKE) {
                return Result(message = "stroke $index points must contain between 1 and $MAX_POINTS_PER_STROKE entries")
            }
            val path = Path()
            for (pointIndex in 0 until points.length()) {
                val point = parsePoint(points.opt(pointIndex))
                    ?: return Result(message = "stroke $index point $pointIndex is invalid")
                if (pointIndex == 0) {
                    path.moveTo(point.first, point.second)
                } else {
                    path.lineTo(point.first, point.second)
                }
            }
            val duration = strokeObject.optLong("durationMs", 300L).coerceIn(1L, MAX_DURATION_MS)
            val startTime = strokeObject.optLong("startTimeMs", 0L).coerceIn(0L, MAX_START_TIME_MS)
            val willContinue = strokeObject.optBoolean("willContinue", false)
            val stroke = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O && willContinue) {
                GestureDescription.StrokeDescription(path, startTime, duration, true)
            } else {
                GestureDescription.StrokeDescription(path, startTime, duration)
            }
            builder.addStroke(stroke)
        }
        return Result(gesture = builder.build())
    }

    private fun parsePoint(value: Any?): Pair<Float, Float>? {
        return when (value) {
            is JSONArray -> {
                if (value.length() < 2) return null
                val x = value.optDouble(0, Double.NaN)
                val y = value.optDouble(1, Double.NaN)
                if (!x.isFinite() || !y.isFinite()) null else x.toFloat() to y.toFloat()
            }
            is JSONObject -> {
                val x = value.optDouble("x", Double.NaN)
                val y = value.optDouble("y", Double.NaN)
                if (!x.isFinite() || !y.isFinite()) null else x.toFloat() to y.toFloat()
            }
            else -> null
        }
    }
}
