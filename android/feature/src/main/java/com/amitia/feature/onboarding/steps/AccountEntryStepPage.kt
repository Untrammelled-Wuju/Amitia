package com.amitia.feature.onboarding.steps

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
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
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaCardShape
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.core.designsystem.component.TertiaryButton

@Composable
fun AccountEntryStepPage(
    onRegister: () -> Unit,
    onLogin: () -> Unit,
    onUseLocal: () -> Unit,
    modifier: Modifier = Modifier
) {
    Surface(
        modifier = modifier.fillMaxSize(),
        color = MaterialTheme.colorScheme.background
    ) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(AmitiaSpacing.Xxl),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Md)
        ) {
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
            CharacterAvatar(name = "Amitia", size = 80)
            StepTitle(text = "账号入口")
            StepDescription(text = "注册新账号、登录已有账号，或在本地单用户模式下使用。")
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            AccountOptionCard(
                title = "注册",
                description = "创建新的 Amitia 账号",
                icon = AmitiaIcons.PersonAdd,
                onClick = onRegister
            )
            AccountOptionCard(
                title = "登录",
                description = "使用已有账号登录",
                icon = AmitiaIcons.Login,
                onClick = onLogin
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xs))
            TertiaryButton(
                text = "使用本地账号",
                onClick = onUseLocal,
                leadingIcon = AmitiaIcons.Storage
            )
        }
    }
}

@Composable
private fun AccountOptionCard(
    title: String,
    description: String,
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    onClick: () -> Unit
) {
    val interactionSource = remember { MutableInteractionSource() }
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .clip(AmitiaCardShape)
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                role = Role.Button,
                onClick = onClick
            )
            .border(1.dp, MaterialTheme.colorScheme.outline, AmitiaCardShape),
        shape = AmitiaCardShape,
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Box(
                modifier = Modifier
                    .size(44.dp)
                    .clip(AmitiaCardShape)
                    .background(MaterialTheme.colorScheme.primaryContainer),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = icon,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onPrimaryContainer,
                    modifier = Modifier.size(22.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = title,
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    fontWeight = FontWeight.Medium
                )
                Text(
                    text = description,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            Icon(
                imageVector = AmitiaIcons.ChevronRight,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f),
                modifier = Modifier.size(24.dp)
            )
        }
    }
}

@Preview(name = "AccountEntry - Light", showBackground = true)
@Composable
private fun AccountEntryStepPageLightPreview() {
    AmitiaTheme(darkTheme = false) {
        AccountEntryStepPage(
            onRegister = {},
            onLogin = {},
            onUseLocal = {}
        )
    }
}

@Preview(name = "AccountEntry - Dark", showBackground = true)
@Composable
private fun AccountEntryStepPageDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        AccountEntryStepPage(
            onRegister = {},
            onLogin = {},
            onUseLocal = {}
        )
    }
}
