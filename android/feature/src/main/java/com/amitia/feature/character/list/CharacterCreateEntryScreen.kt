package com.amitia.feature.character.list

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.AddBox
import androidx.compose.material.icons.outlined.ArrowBack
import androidx.compose.material.icons.outlined.AutoAwesome
import androidx.compose.material.icons.outlined.CloudUpload
import androidx.compose.material.icons.outlined.Dashboard
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaTopBar

enum class CreateEntryType(val title: String, val subtitle: String, val icon: ImageVector) {
    Blank("从零创建", "从空白开始，逐步设定角色的一切", Icons.Outlined.AddBox),
    Template("从模板创建", "基于预设模板快速生成，再按需调整", Icons.Outlined.Dashboard),
    Import("导入角色包", "从导出的角色包文件恢复角色", Icons.Outlined.CloudUpload),
    Image("从图片生成", "上传一张图片，自动生成角色形象", Icons.Outlined.AutoAwesome)
}

@Composable
fun CharacterCreateEntryScreen(
    onBack: () -> Unit,
    onSelectBlank: () -> Unit,
    onSelectTemplate: () -> Unit,
    onSelectImport: () -> Unit,
    onSelectImage: () -> Unit
) {
    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            AmitiaTopBar(
                title = "创建角色",
                onBack = onBack
            )
        }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 20.dp, vertical = 16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Text(
                text = "选择创建方式",
                style = MaterialTheme.typography.titleMedium,
                color = MaterialTheme.colorScheme.onSurface
            )
            Text(
                text = "不同的方式适合不同的需求，创建后随时可以调整",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Spacer(modifier = Modifier.height(4.dp))
            CreateEntryCard(
                entry = CreateEntryType.Blank,
                onClick = onSelectBlank,
                isRecommended = true
            )
            CreateEntryCard(
                entry = CreateEntryType.Template,
                onClick = onSelectTemplate
            )
            CreateEntryCard(
                entry = CreateEntryType.Import,
                onClick = onSelectImport
            )
            CreateEntryCard(
                entry = CreateEntryType.Image,
                onClick = onSelectImage
            )
        }
    }
}

@Composable
private fun CreateEntryCard(
    entry: CreateEntryType,
    onClick: () -> Unit,
    isRecommended: Boolean = false
) {
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick),
        shape = RoundedCornerShape(20.dp),
        color = if (isRecommended) MaterialTheme.colorScheme.primaryContainer
        else MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(16.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(16.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(48.dp)
                    .clip(CircleShape)
                    .background(
                        if (isRecommended) MaterialTheme.colorScheme.primary
                        else MaterialTheme.colorScheme.surfaceVariant
                    ),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = entry.icon,
                    contentDescription = null,
                    tint = if (isRecommended) MaterialTheme.colorScheme.onPrimary
                    else MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.size(24.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        text = entry.title,
                        style = MaterialTheme.typography.titleMedium,
                        color = if (isRecommended) MaterialTheme.colorScheme.onPrimaryContainer
                        else MaterialTheme.colorScheme.onSurface,
                        fontWeight = FontWeight.Medium
                    )
                    if (isRecommended) {
                        Spacer(modifier = Modifier.size(8.dp))
                        Surface(
                            shape = CircleShape,
                            color = MaterialTheme.colorScheme.primary
                        ) {
                            Text(
                                text = "推荐",
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.onPrimary,
                                modifier = Modifier.padding(horizontal = 8.dp, vertical = 2.dp)
                            )
                        }
                    }
                }
                Text(
                    text = entry.subtitle,
                    style = MaterialTheme.typography.bodySmall,
                    color = if (isRecommended) MaterialTheme.colorScheme.onPrimaryContainer.copy(alpha = 0.7f)
                    else MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis
                )
            }
        }
    }
}

@Preview(name = "Create Entry - Light", showBackground = true)
@Composable
private fun CharacterCreateEntryLightPreview() {
    AmitiaTheme(darkTheme = false) {
        CharacterCreateEntryScreen(
            onBack = {},
            onSelectBlank = {},
            onSelectTemplate = {},
            onSelectImport = {},
            onSelectImage = {}
        )
    }
}

@Preview(name = "Create Entry - Dark", showBackground = true)
@Composable
private fun CharacterCreateEntryDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        CharacterCreateEntryScreen(
            onBack = {},
            onSelectBlank = {},
            onSelectTemplate = {},
            onSelectImport = {},
            onSelectImage = {}
        )
    }
}
