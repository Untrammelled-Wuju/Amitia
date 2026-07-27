package com.amitia.feature.chat

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaBottomSheet
import com.amitia.core.designsystem.component.PermissionRequiredState

private val attachmentOptions = listOf(
    AttachmentOption("image", "图片", AttachmentIcon.Image, enabled = true),
    AttachmentOption("camera", "相机", AttachmentIcon.Camera, enabled = true),
    AttachmentOption("file", "文件", AttachmentIcon.File, enabled = true),
    AttachmentOption("memory", "记忆", AttachmentIcon.Memory, enabled = true),
    AttachmentOption("location", "位置", AttachmentIcon.Location, enabled = false, permissionRequired = true),
    AttachmentOption("contact", "联系人", AttachmentIcon.Contact, enabled = false, permissionRequired = true)
)

@Composable
fun AttachmentSheet(
    onDismiss: () -> Unit,
    onPickImage: () -> Unit,
    onTakePhoto: () -> Unit,
    onPickFile: () -> Unit,
    onPickMemory: () -> Unit
) {
    AmitiaBottomSheet(onDismiss = onDismiss, title = "发送附件") {
        LazyVerticalGrid(
            columns = GridCells.Fixed(3),
            modifier = Modifier.fillMaxWidth(),
            contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            items(attachmentOptions, key = { it.id }) { option ->
                AttachmentOptionItem(option = option) {
                    when (option.id) {
                        "image" -> onPickImage()
                        "camera" -> onTakePhoto()
                        "file" -> onPickFile()
                        "memory" -> onPickMemory()
                    }
                    if (option.enabled) onDismiss()
                }
            }
        }
    }
}

@Composable
private fun AttachmentOptionItem(
    option: AttachmentOption,
    onClick: () -> Unit
) {
    val interactionSource = remember { MutableInteractionSource() }
    val icon = attachmentIcon(option.iconType)
    val tint = if (option.enabled) MaterialTheme.colorScheme.onPrimaryContainer
    else MaterialTheme.colorScheme.onSurfaceVariant
    val bgColor = if (option.enabled) MaterialTheme.colorScheme.primaryContainer
    else MaterialTheme.colorScheme.surfaceVariant

    Column(
        modifier = Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(16.dp))
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                enabled = option.enabled,
                role = Role.Button,
                onClick = onClick
            )
            .padding(vertical = AmitiaSpacing.Sm),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
    ) {
        Box(
            modifier = Modifier
                .size(56.dp)
                .clip(CircleShape)
                .background(bgColor),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = icon,
                contentDescription = option.label,
                tint = tint,
                modifier = Modifier.size(AmitiaIconSize.Large)
            )
        }
        Text(
            text = option.label,
            style = MaterialTheme.typography.labelMedium,
            color = if (option.enabled) MaterialTheme.colorScheme.onSurface
            else MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f),
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            textAlign = TextAlign.Center
        )
        if (option.permissionRequired) {
            Text(
                text = "需授权",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.error.copy(alpha = 0.7f)
            )
        }
    }
}

private fun attachmentIcon(type: AttachmentIcon): ImageVector = when (type) {
    AttachmentIcon.Image -> AmitiaIcons.Image
    AttachmentIcon.Camera -> AmitiaIcons.Camera
    AttachmentIcon.File -> AmitiaIcons.AttachFile
    AttachmentIcon.Memory -> AmitiaIcons.Memory
    AttachmentIcon.Location -> AmitiaIcons.LocationOn
    AttachmentIcon.Contact -> AmitiaIcons.PersonAdd
}

@Preview(name = "Attachment Sheet - Light", showBackground = true)
@Composable
private fun AttachmentSheetLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
            attachmentOptions.take(3).forEach { option ->
                Surface(
                    shape = RoundedCornerShape(16.dp),
                    color = MaterialTheme.colorScheme.surface,
                    modifier = Modifier.fillMaxWidth().padding(vertical = 4.dp)
                ) {
                    Box(modifier = Modifier.padding(AmitiaSpacing.Base)) {
                        AttachmentOptionItem(option = option, onClick = {})
                    }
                }
            }
        }
    }
}

@Preview(name = "Attachment Sheet - Dark", showBackground = true)
@Composable
private fun AttachmentSheetDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        PermissionRequiredState(
            permissionName = "位置",
            description = "发送位置需要位置权限",
            onGrant = {}
        )
    }
}
