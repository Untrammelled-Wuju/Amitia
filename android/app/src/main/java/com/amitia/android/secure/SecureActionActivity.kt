package com.amitia.android.secure

import android.app.Activity
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import com.amitia.android.navigation.AmitiaRoutes
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaSpacing
import dagger.hilt.android.AndroidEntryPoint

@AndroidEntryPoint
class SecureActionActivity : ComponentActivity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        val startRoute = intent.getStringExtra(EXTRA_ROUTE) ?: AmitiaRoutes.Secure.APP_UNLOCK
        setContent {
            AmitiaTheme {
                Surface(modifier = Modifier.fillMaxSize()) {
                    SecureActionNavHost(
                        startRoute = startRoute,
                        onConfirmed = { confirmed() },
                        onCanceled = { canceled() }
                    )
                }
            }
        }
    }

    private fun confirmed() {
        setResult(Activity.RESULT_OK)
        finish()
    }

    private fun canceled() {
        setResult(Activity.RESULT_CANCELED)
        finish()
    }

    companion object {
        const val EXTRA_ROUTE = "secure_route"
    }
}

@Composable
private fun SecureActionNavHost(
    startRoute: String,
    onConfirmed: () -> Unit,
    onCanceled: () -> Unit
) {
    val navController = rememberNavController()
    NavHost(
        navController = navController,
        startDestination = startRoute
    ) {
        composable(AmitiaRoutes.Secure.APP_UNLOCK) {
            SecureScreen(
                title = "应用锁验证",
                description = "请完成二次验证以解锁应用，可使用指纹、面部或 PIN 码。",
                confirmLabel = "解锁",
                onConfirm = onConfirmed,
                onCancel = onCanceled
            )
        }

        composable(AmitiaRoutes.Secure.COMPUTER_USE_APPROVAL) {
            SecureScreen(
                title = "Computer Use 审批",
                description = "Amitia 请求执行计算机操作，请确认授权该高风险操作。",
                confirmLabel = "授权执行",
                onConfirm = onConfirmed,
                onCancel = onCanceled
            )
        }

        composable(AmitiaRoutes.Secure.SENSITIVE_PERMISSION) {
            SecureScreen(
                title = "敏感权限确认",
                description = "Amitia 请求授予敏感权限，请确认是否允许。",
                confirmLabel = "允许",
                onConfirm = onConfirmed,
                onCancel = onCanceled
            )
        }

        composable(AmitiaRoutes.Secure.DESTRUCTIVE_CONFIRMATION) {
            SecureScreen(
                title = "破坏性操作确认",
                description = "此操作不可撤销，将永久删除相关数据，请谨慎确认。",
                confirmLabel = "确认删除",
                onConfirm = onConfirmed,
                onCancel = onCanceled
            )
        }
    }
}

@Composable
private fun SecureScreen(
    title: String,
    description: String,
    confirmLabel: String,
    onConfirm: () -> Unit,
    onCancel: () -> Unit
) {
    Box(
        modifier = Modifier.fillMaxSize(),
        contentAlignment = Alignment.Center
    ) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Xxl),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Lg)
        ) {
            Text(
                text = title,
                style = MaterialTheme.typography.headlineMedium,
                color = MaterialTheme.colorScheme.onSurface,
                textAlign = TextAlign.Center
            )
            Text(
                text = description,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                textAlign = TextAlign.Center
            )
            Column(
                modifier = Modifier.padding(top = AmitiaSpacing.Lg),
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Button(onClick = onConfirm, modifier = Modifier.padding(horizontal = AmitiaSpacing.None)) {
                    Text(confirmLabel)
                }
                OutlinedButton(onClick = onCancel) {
                    Text("取消")
                }
            }
        }
    }
}
