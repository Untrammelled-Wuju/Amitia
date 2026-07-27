package com.amitia.feature.chat

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaCardShape
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaContentSurface
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.DangerButton
import com.amitia.core.designsystem.component.LoadingSkeleton
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.core.designsystem.component.StatusRow
import com.amitia.core.designsystem.component.amitiaStatusColor
import com.amitia.core.designsystem.component.amitiaStatusText

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun FileDetailScreen(
    fileId: String,
    onBack: () -> Unit,
    onDownload: (ChatFileInfo) -> Unit,
    onShare: (ChatFileInfo) -> Unit,
    onReUpload: (ChatFileInfo) -> Unit,
    viewModel: ChatExtendViewModel = hiltViewModel()
) {
    LaunchedEffect(fileId) { viewModel.loadFileDetail(fileId) }
    val state by viewModel.fileDetailState.collectAsStateWithLifecycle()

    Column(modifier = Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        TopAppBar(
            title = { Text(text = "文件详情", style = MaterialTheme.typography.titleLarge) },
            navigationIcon = {
                AmitiaIconButton(icon = AmitiaIcons.ArrowBack, contentDescription = "返回", onClick = onBack)
            },
            colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.background)
        )
        when (val s = state) {
            is ScreenState.Loading -> LoadingSkeleton(lineCount = 6, lineHeight = 48)
            is ScreenState.Error -> AmitiaErrorState(
                icon = AmitiaIcons.ErrorOutline,
                title = s.error.title,
                description = s.error.message,
                onRetry = { viewModel.loadFileDetail(fileId) }
            )
            is ScreenState.Empty -> AmitiaEmptyState(
                icon = AmitiaIcons.FileCopy,
                title = "文件不存在",
                description = "该文件可能已被删除"
            )
            is ScreenState.Content, is ScreenState.Partial -> {
                val file = (s as ScreenState.Content<ChatFileInfo>).data
                FileDetailContent(
                    file = file,
                    onDownload = { onDownload(file) },
                    onShare = { onShare(file) },
                    onReUpload = { onReUpload(file) }
                )
            }
        }
    }
}

@Composable
private fun FileDetailContent(
    file: ChatFileInfo,
    onDownload: () -> Unit,
    onShare: () -> Unit,
    onReUpload: () -> Unit
) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
    ) {
        item(key = "file_icon_card") {
            FileIconCard(file = file)
        }
        item(key = "file_info_section") {
            AmitiaSection(title = "文件信息") {
                AmitiaContentSurface {
                    Column {
                        FileInfoRow(label = "文件名", value = file.name)
                        FileInfoRow(label = "类型", value = file.mimeType)
                        FileInfoRow(label = "大小", value = formatFileSize(file.sizeBytes))
                        FileInfoRow(label = "上传时间", value = file.uploadedAt)
                        Row(
                            modifier = Modifier.fillMaxWidth().padding(AmitiaSpacing.Base),
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Text(
                                text = "上传状态",
                                style = MaterialTheme.typography.bodyMedium,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                            Spacer(modifier = Modifier.weight(1f))
                            Box(
                                modifier = Modifier.size(8.dp).clip(CircleShape)
                                    .background(amitiaStatusColor(file.uploadStatus.status))
                            )
                            Spacer(modifier = Modifier.width(AmitiaSpacing.Xs))
                            Text(
                                text = file.uploadStatus.label,
                                style = MaterialTheme.typography.labelMedium,
                                color = amitiaStatusColor(file.uploadStatus.status)
                            )
                        }
                    }
                }
            }
        }
        item(key = "model_read_section") {
            AmitiaSection(title = "模型读取") {
                AmitiaContentSurface {
                    StatusRow(
                        title = "模型是否已读取",
                        status = if (file.modelRead) com.amitia.core.designsystem.component.AmitiaStatusType.Running
                        else com.amitia.core.designsystem.component.AmitiaStatusType.Idle,
                        subtitle = if (file.modelRead) "该文件内容已注入对话上下文" else "该文件尚未被模型读取",
                        leadingIcon = AmitiaIcons.Psychology
                    )
                }
            }
        }
        if (file.uploadStatus == FileUploadStatus.Failed) {
            item(key = "failed_actions") {
                PrimaryButton(
                    text = "重新上传",
                    onClick = onReUpload,
                    modifier = Modifier.fillMaxWidth(),
                    leadingIcon = AmitiaIcons.CloudUpload
                )
            }
        }
        item(key = "file_actions") {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                SecondaryButton(
                    text = "分享",
                    onClick = onShare,
                    modifier = Modifier.weight(1f),
                    leadingIcon = AmitiaIcons.Share
                )
                PrimaryButton(
                    text = "下载",
                    onClick = onDownload,
                    modifier = Modifier.weight(1f),
                    leadingIcon = AmitiaIcons.Download
                )
            }
        }
    }
}

@Composable
private fun FileIconCard(file: ChatFileInfo) {
    AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
        Column(
            modifier = Modifier.fillMaxWidth().padding(AmitiaSpacing.Lg),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Box(
                modifier = Modifier.size(72.dp).clip(RoundedCornerShape(16.dp))
                    .background(MaterialTheme.colorScheme.primaryContainer),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = fileIconFor(file.mimeType),
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onPrimaryContainer,
                    modifier = Modifier.size(36.dp)
                )
            }
            Text(
                text = file.name,
                style = MaterialTheme.typography.titleMedium,
                color = MaterialTheme.colorScheme.onSurface,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis
            )
            Text(
                text = formatFileSize(file.sizeBytes),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Composable
private fun FileInfoRow(label: String, value: String) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Text(
            text = label,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        Spacer(modifier = Modifier.weight(1f))
        Text(
            text = value,
            style = MaterialTheme.typography.bodyMedium.copy(fontFamily = FontFamily.Monospace),
            color = MaterialTheme.colorScheme.onSurface,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis
        )
    }
}

private fun fileIconFor(mimeType: String) = when {
    mimeType.startsWith("image/") -> AmitiaIcons.Image
    mimeType.startsWith("audio/") -> AmitiaIcons.MusicNote
    mimeType.startsWith("video/") -> AmitiaIcons.PlayArrow
    mimeType == "application/pdf" -> AmitiaIcons.FileCopy
    else -> AmitiaIcons.AttachFile
}

private fun formatFileSize(bytes: Long): String {
    if (bytes < 1024) return "${bytes}B"
    val kb = bytes / 1024.0
    if (kb < 1024) return "${String.format("%.1f", kb)}KB"
    val mb = kb / 1024.0
    return "${String.format("%.2f", mb)}MB"
}

@Preview(name = "File Detail - Light", showBackground = true)
@Composable
private fun FileDetailLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
                FileIconCard(
                    file = ChatFileInfo(
                        id = "f1", name = "项目文档.pdf", mimeType = "application/pdf",
                        sizeBytes = 1258291, url = "", uploadedAt = "今天 14:00",
                        uploadStatus = FileUploadStatus.Uploaded, modelRead = true
                    )
                )
            }
        }
    }
}

@Preview(name = "File Detail - Dark", showBackground = true)
@Composable
private fun FileDetailDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
                FileIconCard(
                    file = ChatFileInfo(
                        id = "f1", name = "项目文档.pdf", mimeType = "application/pdf",
                        sizeBytes = 1258291, url = "", uploadedAt = "今天 14:00",
                        uploadStatus = FileUploadStatus.Uploaded, modelRead = true
                    )
                )
            }
        }
    }
}
