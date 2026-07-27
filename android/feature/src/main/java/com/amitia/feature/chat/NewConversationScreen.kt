package com.amitia.feature.chat

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
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
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaSelectionRow
import com.amitia.core.designsystem.component.CharacterCard
import com.amitia.core.designsystem.component.LoadingButton
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.amitiaStatusColor
import com.amitia.core.designsystem.component.amitiaStatusText
import com.amitia.core.model.CharacterDto
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Icon
import androidx.compose.ui.draw.clip
import androidx.compose.ui.semantics.Role
import com.amitia.core.designsystem.component.AmitiaStatusType

@Composable
fun NewConversationScreen(
    onBack: () -> Unit,
    onCreated: (String) -> Unit,
    viewModel: ChatExtendViewModel = hiltViewModel()
) {
    val characters by viewModel.availableCharacters.collectAsStateWithLifecycle()
    val channels by viewModel.availableChannels.collectAsStateWithLifecycle()
    var selectedCharacterId by remember { mutableStateOf<String?>(null) }
    var selectedChannelId by remember { mutableStateOf<String?>(null) }
    var creating by remember { mutableStateOf(false) }

    Column(modifier = Modifier.fillMaxSize()) {
        NewConversationTopBar(onBack = onBack)
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(bottom = AmitiaSpacing.Xxl),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            item(key = "character_header") {
                AmitiaSectionHeader(title = "选择角色", modifier = Modifier.padding(horizontal = AmitiaSpacing.Base))
            }
            items(characters, key = { it.id }) { character ->
                CharacterSelectionItem(
                    character = character,
                    selected = character.id == selectedCharacterId,
                    onClick = { selectedCharacterId = character.id }
                )
            }
            item(key = "channel_header") {
                AmitiaSectionHeader(title = "选择渠道", modifier = Modifier.padding(horizontal = AmitiaSpacing.Base))
            }
            items(channels, key = { it.id }) { channel ->
                AmitiaSelectionRow(
                    title = channel.name,
                    subtitle = if (channel.isLastUsed) "${channel.description} · 上次使用" else channel.description,
                    selected = channel.id == selectedChannelId,
                    enabled = channel.available,
                    leadingIcon = AmitiaIcons.Hub,
                    onSelect = { if (channel.available) selectedChannelId = channel.id }
                )
            }
            item(key = "create_button") {
                Box(modifier = Modifier.padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Lg)) {
                    LoadingButton(
                        text = "开始对话",
                        onClick = {
                            val charId = selectedCharacterId
                            if (charId != null) {
                                creating = true
                                onCreated(charId)
                            }
                        },
                        loading = creating,
                        enabled = selectedCharacterId != null && selectedChannelId != null,
                        modifier = Modifier.fillMaxWidth(),
                        leadingIcon = AmitiaIcons.Chat
                    )
                }
            }
        }
    }
}

@Composable
private fun NewConversationTopBar(onBack: () -> Unit) {
    androidx.compose.material3.TopAppBar(
        title = {
            Text(text = "新建会话", style = MaterialTheme.typography.titleLarge)
        },
        navigationIcon = {
            AmitiaIconButton(
                icon = AmitiaIcons.ArrowBack,
                contentDescription = "返回",
                onClick = onBack
            )
        },
        colors = androidx.compose.material3.TopAppBarDefaults.topAppBarColors(
            containerColor = MaterialTheme.colorScheme.background
        )
    )
}

@Composable
private fun CharacterSelectionItem(
    character: CharacterDto,
    selected: Boolean,
    onClick: () -> Unit
) {
    val interactionSource = remember { MutableInteractionSource() }
    androidx.compose.material3.Surface(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = AmitiaSpacing.Base)
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                role = Role.RadioButton,
                onClick = onClick
            ),
        shape = androidx.compose.foundation.shape.RoundedCornerShape(16.dp),
        color = if (selected) MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.4f)
        else MaterialTheme.colorScheme.surface
    ) {
        androidx.compose.foundation.layout.Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Box(
                modifier = Modifier
                    .size(48.dp)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.tertiaryContainer),
                contentAlignment = Alignment.Center
            ) {
                Text(
                    text = character.name.take(1),
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onTertiaryContainer,
                    fontWeight = FontWeight.Medium
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                androidx.compose.foundation.layout.Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    Text(
                        text = character.name,
                        style = MaterialTheme.typography.titleSmall,
                        color = MaterialTheme.colorScheme.onSurface,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                    if (character.isCurrent) {
                        OnlineStatusBadge()
                    }
                }
                character.description?.let {
                    Text(
                        text = it,
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                }
            }
            Icon(
                imageVector = if (selected) AmitiaIcons.RadioButtonChecked else AmitiaIcons.RadioButtonUnchecked,
                contentDescription = null,
                tint = if (selected) MaterialTheme.colorScheme.primary
                else MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.size(AmitiaIconSize.Medium)
            )
        }
    }
}

@Composable
private fun OnlineStatusBadge() {
    androidx.compose.foundation.layout.Row(
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xxs)
    ) {
        Box(
            modifier = Modifier
                .size(6.dp)
                .clip(CircleShape)
                .background(amitiaStatusColor(AmitiaStatusType.Connected))
        )
        Text(
            text = amitiaStatusText(AmitiaStatusType.Connected),
            style = MaterialTheme.typography.labelSmall,
            color = amitiaStatusColor(AmitiaStatusType.Connected)
        )
    }
}

@Preview(name = "New Conversation - Light", showBackground = true)
@Composable
private fun NewConversationLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
            CharacterCard(
                name = "艾米",
                identity = "温柔知性助手",
                status = AmitiaStatusType.Connected,
                lastInteraction = "在线",
                onClick = {}
            )
        }
    }
}

@Preview(name = "New Conversation - Dark", showBackground = true)
@Composable
private fun NewConversationDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
            AmitiaSelectionRow(
                title = "Web 对话",
                subtitle = "应用内对话 · 上次使用",
                selected = true,
                leadingIcon = AmitiaIcons.Hub,
                onSelect = {}
            )
        }
    }
}
