package com.amitia.amitia_app.notifications

import android.app.Activity
import android.content.Intent
import android.graphics.Color
import android.os.Bundle
import android.view.Gravity
import android.widget.Button
import android.widget.LinearLayout
import android.widget.TextView
import com.amitia.amitia_app.MainActivity

class ConversationBubbleActivity : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        window.statusBarColor = Color.rgb(245, 247, 253)
        window.navigationBarColor = Color.rgb(245, 247, 253)
        val density = resources.displayMetrics.density
        val pad = (20 * density).toInt()
        val title = intent.getStringExtra("title").orEmpty().ifBlank { "Amitia" }
        val message = intent.getStringExtra("body").orEmpty()
        val conversationId = intent.getStringExtra("conversationId").orEmpty()
        val route = intent.getStringExtra("deepLink").orEmpty().ifBlank {
            "amitia://chat/$conversationId"
        }
        val layout = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            gravity = Gravity.CENTER_VERTICAL
            setPadding(pad, pad, pad, pad)
            setBackgroundColor(Color.rgb(245, 247, 253))
        }
        layout.addView(TextView(this).apply {
            text = title
            textSize = 21f
            setTextColor(Color.rgb(32, 42, 66))
        })
        layout.addView(TextView(this).apply {
            text = message
            textSize = 16f
            setPadding(0, pad, 0, pad)
            setTextColor(Color.rgb(74, 82, 99))
        })
        layout.addView(Button(this).apply {
            text = "打开完整会话"
            setOnClickListener {
                val launch = Intent(this@ConversationBubbleActivity, MainActivity::class.java)
                    .setAction(Intent.ACTION_VIEW)
                    .setData(android.net.Uri.parse(route))
                    .putExtra("amitia.deepLink", route)
                    .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP)
                startActivity(launch)
                finish()
            }
        })
        setContentView(layout)
    }
}
