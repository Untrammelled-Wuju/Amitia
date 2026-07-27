package com.amitia.feature.today

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
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
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaRadius
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaBottomSheet
import com.amitia.core.designsystem.component.AmitiaSearchField
import com.amitia.core.designsystem.component.AmitiaStatusDot
import com.amitia.core.model.CharacterDto

@Composable
fun CharacterSwitchSheet(
    characters: List<CharacterDto>,
    currentCharacterId: String?,
    onDismiss: () -> Unit,
    onSelect: (String) -> Unit
) {
    var query by remember { mutableStateOf("") }
    val filtered = remember(query, characters) {
        if (query.isBlank()) characters
        else characters.filter { it.name.contains(query, ignoreCase = true) }
    }
    val sorted = remember(filtered, currentCharacterId) {
        filtered.sortedByDescending { it.isCurrent || it.id == currentCharacterId }
    }
    AmitiaBottomSheet(
        onDismiss = onDismiss,
        title = "切换角色"
    ) {
        Box(modifier = Modifier.padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm)) {
            AmitiaSearchField(
                value = query,
                onValueChange = { query = it },
                onClear = { query = "" },
                placeholder = "搜索角色"
            )
        }
        if (sorted.isEmpty()) {
            Box(
                modifier = Modifier.fillMaxWidth().padding(AmitiaSpacing.Xxl),
                contentAlignment = Alignment.Center
            ) {
                Text(
                    text = "没有找到匹配的角色",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        } else {
            LazyRow(
                modifier = Modifier.fillMaxWidth(),
                contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Md)
            ) {
                items(sorted, key = { it.id }) { character ->
                    CharacterCardHorizontal(
                        character = character,
                        isCurrent = character.id == currentCharacterId || character.isCurrent,
                        onClick = {
                            onSelect(character.id)
                            onDismiss()
                        }
                    )
                }
            }
        }
    }
}

@Composable
private fun CharacterCardHorizontal(
    character: CharacterDto,
    isCurrent: Boolean,
    onClick: () -> Unit
) {
    val interactionSource = remember { MutableInteractionSource() }
    val borderColor = if (isCurrent) MaterialTheme.colorScheme.primary
    else MaterialTheme.colorScheme.outlineVariant
    Surface(
        modifier = Modifier
            .width(140.dp)
            .clip(RoundedCornerShape(AmitiaRadius.L))
            .border(2.dp, borderColor, RoundedCornerShape(AmitiaRadius.L))
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                role = Role.Button,
                onClick = onClick
            ),
        shape = RoundedCornerShape(AmitiaRadius.L),
        color = if (isCurrent) MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.4f)
        else MaterialTheme.colorScheme.surface
    ) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
        ) {
            Box(
                modifier = Modifier
                    .size(56.dp)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.primaryContainer),
                contentAlignment = Alignment.Center
            ) {
                Text(
                    text = character.name.take(1),
                    style = MaterialTheme.typography.titleLarge,
                    color = MaterialTheme.colorScheme.onPrimaryContainer,
                    fontWeight = FontWeight.Medium
                )
            }
            Text(
                text = character.name,
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onSurface,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
            Text(
                text = character.description ?: "未填写身份",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
            if (isCurrent) {
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
                ) {
                    Icon(
                        imageVector = AmitiaIcons.CheckCircle,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.primary,
                        modifier = Modifier.size(AmitiaIconSize.Small)
                    )
                    Text(
                        text = "当前",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.primary
                    )
                }
            } else {
                AmitiaStatusDot(color = AmitiaStateColors.Idle)
            }
        }
    }
}

@Preview(name = "Character Switch - Light", showBackground = true)
@Composable
private fun CharacterSwitchLightPreview() {
    AmitiaTheme(darkTheme = false) {
        CharacterSwitchSheet(
            characters = listOf(
                CharacterDto(id = "1", name = "艾米", isCurrent = true, description = "温柔知性助手"),
                CharacterDto(id = "2", name = "诺亚", description = "理性分析伙伴")
            ),
            currentCharacterId = "1",
            onDismiss = {},
            onSelect = {}
        )
    }
}

@Preview(name = "Character Switch - Dark", showBackground = true)
@Composable
private fun CharacterSwitchDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        CharacterSwitchSheet(
            characters = listOf(
                CharacterDto(id = "1", name = "艾米", isCurrent = true, description = "温柔知性助手")
            ),
            currentCharacterId = "1",
            onDismiss = {},
            onSelect = {}
        )
    }
}
