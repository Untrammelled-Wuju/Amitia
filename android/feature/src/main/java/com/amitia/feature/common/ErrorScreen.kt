package com.amitia.feature.common

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.ArrowBack
import androidx.compose.material.icons.outlined.BugReport
import androidx.compose.material.icons.outlined.CloudOff
import androidx.compose.material.icons.outlined.Refresh
import androidx.compose.material3.Button
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaColors
import com.amitia.core.error.AmitiaError
import com.amitia.core.error.ErrorMapper

@Composable
fun ErrorScreen(
    error: AmitiaError,
    onRetry: () -> Unit,
    onExportDiagnostics: () -> Unit,
    onBack: () -> Unit,
    errorMapper: ErrorMapper = ErrorMapper()
) {
    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("发生错误") },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.Outlined.ArrowBack, contentDescription = "返回")
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.surface
                )
            )
        }
    ) { innerPadding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(innerPadding)
                .verticalScroll(rememberScrollState())
                .padding(24.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            Icon(
                imageVector = pickIcon(error),
                contentDescription = null,
                tint = MaterialTheme.colorScheme.error,
                modifier = Modifier.size(96.dp)
            )
            Spacer(modifier = Modifier.height(24.dp))
            Text(
                text = errorMapper.toUserMessage(error),
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.SemiBold,
                textAlign = TextAlign.Center,
                color = MaterialTheme.colorScheme.onSurface
            )
            Spacer(modifier = Modifier.height(8.dp))
            Text(
                text = "错误代码：${error.code}",
                style = MaterialTheme.typography.bodySmall,
                color = AmitiaColors.OnSurfaceVariant
            )
            Spacer(modifier = Modifier.height(32.dp))
            if (error.retryable) {
                Button(
                    onClick = onRetry,
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Icon(Icons.Outlined.Refresh, contentDescription = null)
                    Spacer(modifier = Modifier.size(8.dp))
                    Text("重试")
                }
                Spacer(modifier = Modifier.height(12.dp))
            }
            OutlinedButton(
                onClick = onExportDiagnostics,
                modifier = Modifier.fillMaxWidth()
            ) {
                Icon(Icons.Outlined.BugReport, contentDescription = null)
                Spacer(modifier = Modifier.size(8.dp))
                Text("导出诊断信息")
            }
            if (error.requiresUserAction) {
                Spacer(modifier = Modifier.height(16.dp))
                Text(
                    text = "此错误需要您手动处理后方可继续使用",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.error,
                    textAlign = TextAlign.Center
                )
            }
        }
    }
}

private fun pickIcon(error: AmitiaError): ImageVector {
    return when (error) {
        is AmitiaError.NetworkUnavailable,
        is AmitiaError.RemoteUnreachable,
        is AmitiaError.StreamDisconnected -> Icons.Outlined.CloudOff
        else -> Icons.Outlined.BugReport
    }
}
