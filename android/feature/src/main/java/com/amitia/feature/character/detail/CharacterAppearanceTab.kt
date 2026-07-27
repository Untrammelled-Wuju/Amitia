package com.amitia.feature.character.detail

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.AddAPhoto
import androidx.compose.material.icons.outlined.Crop
import androidx.compose.material.icons.outlined.Image
import androidx.compose.material.icons.outlined.Palette
import androidx.compose.material.icons.outlined.Pets
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.feature.character.CharacterDetailViewModel
import com.amitia.feature.character.model.AppearanceAsset
import com.amitia.feature.character.model.CharacterThemeColors

@Composable
fun CharacterAppearanceTab(
    characterId: String,
    viewModel: CharacterDetailViewModel,
    contentPadding: PaddingValues
) {
    val state by viewModel.appearanceState.collectAsStateWithLifecycle()
    when (state) {
        ScreenState.Loading -> DetailLoadingBox()
        is ScreenState.Error -> DetailErrorBox(
            message = (state as ScreenState.Error).error.message,
            onRetry = { viewModel.loadAppearance() }
        )
        is ScreenState.Content -> AppearanceContent(
            assets = (state as ScreenState.Content).data,
            modifier = Modifier.padding(contentPadding)
        )
        else -> DetailEmptyBox("暂无数据")
    }
}

@Composable
private fun AppearanceContent(
    assets: List<AppearanceAsset>,
    modifier: Modifier = Modifier
) {
    var selectedColor by remember { mutableStateOf(CharacterThemeColors.options.first()) }
    LazyColumn(
        modifier = modifier.fillMaxSize(),
        contentPadding = PaddingValues(20.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        item(key = "primary") {
            PrimaryAppearanceCard()
        }
        item(key = "crop") {
            ActionRowCard(
                icon = Icons.Outlined.Crop,
                title = "头像裁切",
                subtitle = "调整头像的显示区域",
                onClick = {}
            )
        }
        item(key = "library") {
            AppearanceLibraryCard(assets)
        }
        item(key = "import") {
            ActionRowCard(
                icon = Icons.Outlined.AddAPhoto,
                title = "导入图片",
                subtitle = "从相册或文件导入新的形象素材",
                onClick = {}
            )
        }
        item(key = "pet_summary") {
            PetAssetSummaryCard()
        }
        item(key = "theme_color") {
            ThemeColorCard(
                selectedColor = selectedColor,
                onSelect = { selectedColor = it }
            )
        }
    }
}

@Composable
private fun PrimaryAppearanceCard() {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(20.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(
            modifier = Modifier.padding(20.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(120.dp)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.primaryContainer),
                contentAlignment = Alignment.Center
            ) {
                Text(
                    text = "艾",
                    style = MaterialTheme.typography.headlineLarge,
                    color = MaterialTheme.colorScheme.onPrimaryContainer,
                    fontWeight = FontWeight.Medium
                )
            }
            Text(
                text = "当前主形象",
                style = MaterialTheme.typography.titleMedium,
                color = MaterialTheme.colorScheme.onSurface
            )
            Text(
                text = "点击更换头像",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Composable
private fun ActionRowCard(
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    title: String,
    subtitle: String,
    onClick: () -> Unit
) {
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(16.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(16.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(40.dp)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.surfaceVariant),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = icon,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.size(20.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = title,
                    style = MaterialTheme.typography.bodyLarge,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Text(
                    text = subtitle,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
            }
        }
    }
}

@Composable
private fun AppearanceLibraryCard(assets: List<AppearanceAsset>) {
    AmitiaSection(title = "形象库", subtitle = "${assets.size} 个素材") {
        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            assets.forEach { asset ->
                AssetRow(asset)
            }
        }
    }
}

@Composable
private fun AssetRow(asset: AppearanceAsset) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(12.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(40.dp)
                    .clip(RoundedCornerShape(8.dp))
                    .background(MaterialTheme.colorScheme.surfaceVariant),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = Icons.Outlined.Image,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.size(20.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        text = asset.label,
                        style = MaterialTheme.typography.bodyLarge,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                    if (asset.isPrimary) {
                        Spacer(modifier = Modifier.size(8.dp))
                        Surface(
                            shape = CircleShape,
                            color = MaterialTheme.colorScheme.primary
                        ) {
                            Text(
                                text = "主形象",
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.onPrimary,
                                modifier = Modifier.padding(horizontal = 6.dp, vertical = 1.dp)
                            )
                        }
                    }
                }
                Text(
                    text = when (asset.type) {
                        com.amitia.feature.character.model.AppearanceAssetType.Avatar -> "头像"
                        com.amitia.feature.character.model.AppearanceAssetType.FullBody -> "立绘"
                        com.amitia.feature.character.model.AppearanceAssetType.Expression -> "表情"
                    },
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }
    }
}

@Composable
private fun PetAssetSummaryCard() {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(16.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(16.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(40.dp)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.tertiaryContainer),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = Icons.Outlined.Pets,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onTertiaryContainer,
                    modifier = Modifier.size(20.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = "桌宠动作资源",
                    style = MaterialTheme.typography.bodyLarge,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Text(
                    text = "6 个已就绪 · 4 个待生成",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            SecondaryButton(text = "管理", onClick = {})
        }
    }
}

@Composable
private fun ThemeColorCard(
    selectedColor: com.amitia.feature.character.model.CharacterThemeColor,
    onSelect: (com.amitia.feature.character.model.CharacterThemeColor) -> Unit
) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Icon(
                    imageVector = Icons.Outlined.Palette,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.size(18.dp)
                )
                Spacer(modifier = Modifier.size(8.dp))
                Text(
                    text = "角色主题色",
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            Spacer(modifier = Modifier.height(12.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                CharacterThemeColors.options.forEach { color ->
                    val isSelected = color.id == selectedColor.id
                    Box(
                        modifier = Modifier
                            .size(40.dp)
                            .clip(CircleShape)
                            .background(color.color)
                            .clickable { onSelect(color) },
                        contentAlignment = Alignment.Center
                    ) {
                        if (isSelected) {
                            Box(
                                modifier = Modifier
                                    .size(16.dp)
                                    .clip(CircleShape)
                                    .background(Color.White.copy(alpha = 0.8f))
                            )
                        }
                    }
                }
            }
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text = "当前：${selectedColor.name}",
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Preview(name = "Appearance - Light", showBackground = true)
@Composable
private fun CharacterAppearanceLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(color = MaterialTheme.colorScheme.background) {
            AppearanceContent(
                assets = listOf(
                    AppearanceAsset("1", com.amitia.feature.character.model.AppearanceAssetType.Avatar, null, "默认头像", true),
                    AppearanceAsset("2", com.amitia.feature.character.model.AppearanceAssetType.FullBody, null, "立绘", false)
                )
            )
        }
    }
}

@Preview(name = "Appearance - Dark", showBackground = true)
@Composable
private fun CharacterAppearanceDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(color = MaterialTheme.colorScheme.background) {
            AppearanceContent(
                assets = listOf(
                    AppearanceAsset("1", com.amitia.feature.character.model.AppearanceAssetType.Avatar, null, "默认头像", true)
                )
            )
        }
    }
}
