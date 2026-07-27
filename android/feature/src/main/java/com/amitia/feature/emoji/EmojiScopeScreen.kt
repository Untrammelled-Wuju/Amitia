package com.amitia.feature.emoji

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
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
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaSelectionRow
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun EmojiScopeScreen(
    onBack: () -> Unit,
    onSave: () -> Unit,
    viewModel: EmojiViewModel = hiltViewModel()
) {
    val scope by viewModel.scopeState.collectAsStateWithLifecycle()
    val characters by viewModel.characters.collectAsStateWithLifecycle()
    EmojiScopeContent(
        scope = scope,
        characters = characters,
        onScopeTypeChange = { type -> viewModel.updateScope { it.copy(scopeType = type) } },
        onToggleCharacter = { id -> viewModel.toggleCharacterSelection(id) },
        onBack = onBack,
        onSave = onSave
    )
}

@Composable
fun EmojiScopeContent(
    scope: EmojiScopeConfig,
    characters: List<CharacterOption>,
    onScopeTypeChange: (EmojiScopeType) -> Unit,
    onToggleCharacter: (String) -> Unit,
    onBack: () -> Unit,
    onSave: () -> Unit
) {
    val showCharacterList = scope.scopeType != EmojiScopeType.AllCharacters
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "角色适用范围", onBack = onBack)
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            item(key = "scope_types") {
                AmitiaSectionHeader(title = "范围类型")
            }
            item(key = "all") {
                AmitiaSelectionRow(
                    title = EmojiScopeType.AllCharacters.label,
                    subtitle = "所有角色都可以使用",
                    selected = scope.scopeType == EmojiScopeType.AllCharacters,
                    onSelect = { onScopeTypeChange(EmojiScopeType.AllCharacters) },
                    leadingIcon = AmitiaIcons.People
                )
            }
            item(key = "only") {
                AmitiaSelectionRow(
                    title = EmojiScopeType.OnlySpecified.label,
                    subtitle = "仅选中的角色可以使用",
                    selected = scope.scopeType == EmojiScopeType.OnlySpecified,
                    onSelect = { onScopeTypeChange(EmojiScopeType.OnlySpecified) },
                    leadingIcon = AmitiaIcons.Person
                )
            }
            item(key = "exclude") {
                AmitiaSelectionRow(
                    title = EmojiScopeType.ExcludeSpecified.label,
                    subtitle = "排除选中的角色，其他角色可用",
                    selected = scope.scopeType == EmojiScopeType.ExcludeSpecified,
                    onSelect = { onScopeTypeChange(EmojiScopeType.ExcludeSpecified) },
                    leadingIcon = AmitiaIcons.PersonAdd
                )
            }

            if (showCharacterList) {
                item(key = "char_header") {
                    AmitiaSectionHeader(title = "选择角色")
                }
                items(characters, key = { it.id }) { character ->
                    CharacterSelectionRow(
                        character = character,
                        onToggle = { onToggleCharacter(character.id) }
                    )
                }
            }

            item(key = "save") {
                Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
                PrimaryButton(
                    text = "保存",
                    onClick = onSave,
                    modifier = Modifier.fillMaxWidth(),
                    leadingIcon = AmitiaIcons.Check
                )
            }
        }
    }
}

@Composable
private fun CharacterSelectionRow(
    character: CharacterOption,
    onToggle: () -> Unit
) {
    val interactionSource = remember { MutableInteractionSource() }
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                role = Role.Checkbox,
                onClick = onToggle
            ),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Box(
                modifier = Modifier
                    .size(40.dp)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.primaryContainer),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = AmitiaIcons.Person,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onPrimaryContainer,
                    modifier = Modifier.size(AmitiaIconSize.Medium)
                )
            }
            Text(
                text = character.name,
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurface,
                modifier = Modifier.weight(1f),
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
            Icon(
                imageVector = if (character.selected) AmitiaIcons.CheckBox else AmitiaIcons.CheckBoxOutlineBlank,
                contentDescription = null,
                tint = if (character.selected) MaterialTheme.colorScheme.primary
                else MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.size(AmitiaIconSize.Medium)
            )
        }
    }
}

@Preview(name = "Emoji Scope - Light", showBackground = true)
@Composable
private fun EmojiScopeLightPreview() {
    AmitiaTheme(darkTheme = false) {
        EmojiScopeContent(
            scope = EmojiScopeConfig(scopeType = EmojiScopeType.OnlySpecified),
            characters = listOf(
                CharacterOption("1", "艾米", selected = true),
                CharacterOption("2", "露娜", selected = false),
                CharacterOption("3", "小薇", selected = true)
            ),
            onScopeTypeChange = {},
            onToggleCharacter = {},
            onBack = {},
            onSave = {}
        )
    }
}

@Preview(name = "Emoji Scope - Dark", showBackground = true)
@Composable
private fun EmojiScopeDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        EmojiScopeContent(
            scope = EmojiScopeConfig(scopeType = EmojiScopeType.AllCharacters),
            characters = emptyList(),
            onScopeTypeChange = {},
            onToggleCharacter = {},
            onBack = {},
            onSave = {}
        )
    }
}
