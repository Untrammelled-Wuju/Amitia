package com.amitia.feature.voice

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaBottomSheet
import com.amitia.core.designsystem.component.AudioDeviceItem
import com.amitia.core.designsystem.component.AudioDeviceType

@Composable
fun AudioDeviceSheet(
    onDismiss: () -> Unit,
    devices: List<AudioDeviceItem>,
    selectedDeviceId: String?,
    onDeviceSelected: (AudioDeviceItem) -> Unit
) {
    AudioDeviceSheetContent(
        onDismiss = onDismiss,
        devices = devices,
        selectedDeviceId = selectedDeviceId,
        onDeviceSelected = onDeviceSelected
    )
}

@Composable
fun AudioDeviceSheetContent(
    onDismiss: () -> Unit,
    devices: List<AudioDeviceItem>,
    selectedDeviceId: String?,
    onDeviceSelected: (AudioDeviceItem) -> Unit
) {
    AmitiaBottomSheet(
        onDismiss = onDismiss,
        title = "选择音频设备"
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
        ) {
            Text(
                text = "设备列表反映当前系统真实路由",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(bottom = AmitiaSpacing.Sm)
            )
            devices.forEach { device ->
                AudioDeviceRow(
                    device = device,
                    isSelected = device.id == selectedDeviceId,
                    onClick = {
                        onDeviceSelected(device)
                        onDismiss()
                    }
                )
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Base))
        }
    }
}

@Composable
private fun AudioDeviceRow(
    device: AudioDeviceItem,
    isSelected: Boolean,
    onClick: () -> Unit
) {
    val interactionSource = remember { MutableInteractionSource() }
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .height(56.dp)
            .clip(MaterialTheme.shapes.medium)
            .background(
                if (isSelected) MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.5f)
                else MaterialTheme.colorScheme.surface
            )
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                role = Role.RadioButton,
                enabled = device.isConnected,
                onClick = onClick
            )
            .padding(horizontal = AmitiaSpacing.Base),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
    ) {
        Box(
            modifier = Modifier
                .size(AmitiaIconSize.Large)
                .clip(CircleShape)
                .background(
                    if (isSelected) MaterialTheme.colorScheme.primary
                    else MaterialTheme.colorScheme.surfaceVariant
                ),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = audioDeviceIcon(device.type),
                contentDescription = null,
                tint = if (isSelected) MaterialTheme.colorScheme.onPrimary
                else MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.size(AmitiaIconSize.Medium)
            )
        }
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = device.name,
                style = MaterialTheme.typography.bodyLarge,
                color = if (!device.isConnected) MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f)
                else if (isSelected) MaterialTheme.colorScheme.primary
                else MaterialTheme.colorScheme.onSurface,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
            Text(
                text = deviceTypeLabel(device.type) + if (!device.isConnected) " · 未连接" else "",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.7f)
            )
        }
        if (isSelected) {
            Icon(
                imageVector = AmitiaIcons.Check,
                contentDescription = "已选择",
                tint = MaterialTheme.colorScheme.primary,
                modifier = Modifier.size(AmitiaIconSize.Medium)
            )
        }
    }
}

private fun deviceTypeLabel(type: AudioDeviceType): String = when (type) {
    AudioDeviceType.Earpiece -> "手机听筒"
    AudioDeviceType.Speakerphone -> "扬声器"
    AudioDeviceType.Bluetooth -> "蓝牙设备"
    AudioDeviceType.WiredHeadset -> "有线设备"
}

private fun audioDeviceIcon(type: AudioDeviceType): ImageVector = when (type) {
    AudioDeviceType.Earpiece -> AmitiaIcons.PhoneAndroid
    AudioDeviceType.Speakerphone -> AmitiaIcons.VolumeUp
    AudioDeviceType.Bluetooth -> AmitiaIcons.Sensors
    AudioDeviceType.WiredHeadset -> AmitiaIcons.Mic
}

@Preview(name = "Audio Device Sheet - Light", showBackground = true)
@Composable
private fun AudioDeviceSheetLightPreview() {
    AmitiaTheme(darkTheme = false) {
        AudioDeviceSheetContent(
            onDismiss = {},
            devices = listOf(
                AudioDeviceItem("1", "手机听筒", AudioDeviceType.Earpiece),
                AudioDeviceItem("2", "扬声器", AudioDeviceType.Speakerphone),
                AudioDeviceItem("3", "蓝牙耳机 AirPods Pro", AudioDeviceType.Bluetooth),
                AudioDeviceItem("4", "有线耳机", AudioDeviceType.WiredHeadset, isConnected = false)
            ),
            selectedDeviceId = "2",
            onDeviceSelected = {}
        )
    }
}

@Preview(name = "Audio Device Sheet - Dark", showBackground = true)
@Composable
private fun AudioDeviceSheetDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        AudioDeviceSheetContent(
            onDismiss = {},
            devices = listOf(
                AudioDeviceItem("1", "手机听筒", AudioDeviceType.Earpiece),
                AudioDeviceItem("2", "扬声器", AudioDeviceType.Speakerphone)
            ),
            selectedDeviceId = "1",
            onDeviceSelected = {}
        )
    }
}
