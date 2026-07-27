package com.amitia.feature.computeruse

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
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
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading
import com.amitia.core.designsystem.component.WarningBanner

@Composable
fun SafetyRulesScreen(
    onBack: () -> Unit,
    viewModel: ComputerUseViewModel = hiltViewModel()
) {
    val rules by viewModel.safetyRules.collectAsStateWithLifecycle()
    val loading by viewModel.loading.collectAsStateWithLifecycle()
    SafetyRulesContent(
        rules = rules,
        loading = loading,
        onBack = onBack,
        onToggle = viewModel::toggleSafetyRule,
        onMoveUp = viewModel::moveRuleUp,
        onMoveDown = viewModel::moveRuleDown
    )
}

@Composable
fun SafetyRulesContent(
    rules: List<SafetyRule>,
    loading: Boolean,
    onBack: () -> Unit,
    onToggle: (String, Boolean) -> Unit,
    onMoveUp: (String) -> Unit,
    onMoveDown: (String) -> Unit
) {
    val sortedRules = rules.sortedBy { it.priority }
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "安全规则", onBack = onBack)
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            item(key = "warn") {
                WarningBanner(message = "安全规则按优先级从高到低执行，高优先级规则优先生效")
            }
            if (loading) {
                item(key = "loading") {
                    Box(modifier = Modifier.fillMaxWidth().padding(AmitiaSpacing.Xl), contentAlignment = Alignment.Center) {
                        InlineLoading(message = "加载安全规则...")
                    }
                }
            } else if (sortedRules.isEmpty()) {
                item(key = "empty") {
                    AmitiaEmptyState(
                        icon = AmitiaIcons.Shield,
                        title = "暂无安全规则",
                        description = "配置安全规则可限制 Computer Use 的操作范围",
                        modifier = Modifier.fillMaxWidth()
                    )
                }
            } else {
                item(key = "header") {
                    AmitiaSectionHeader(title = "规则列表（按优先级）", trailing = {
                        Text(
                            text = "${sortedRules.size} 条",
                            style = MaterialTheme.typography.labelMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    })
                }
                itemsIndexed(sortedRules, key = { _, rule -> rule.id }) { index, rule ->
                    SafetyRuleCard(
                        rule = rule,
                        isFirst = index == 0,
                        isLast = index == sortedRules.lastIndex,
                        onToggle = { onToggle(rule.id, it) },
                        onMoveUp = { onMoveUp(rule.id) },
                        onMoveDown = { onMoveDown(rule.id) }
                    )
                }
            }
        }
    }
}

@Composable
private fun SafetyRuleCard(
    rule: SafetyRule,
    isFirst: Boolean,
    isLast: Boolean,
    onToggle: (Boolean) -> Unit,
    onMoveUp: () -> Unit,
    onMoveDown: () -> Unit
) {
    val (icon, accentColor) = ruleIconAndColor(rule.type)
    val cardColor = if (rule.enabled) MaterialTheme.colorScheme.surface
    else MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.5f)
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        color = cardColor
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base), verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
            ) {
                Surface(shape = MaterialTheme.shapes.small, color = accentColor.copy(alpha = 0.2f)) {
                    Text(
                        text = "P${rule.priority}",
                        style = MaterialTheme.typography.labelSmall,
                        color = accentColor,
                        fontWeight = FontWeight.Medium,
                        modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 2.dp)
                    )
                }
                Box(
                    modifier = Modifier.size(36.dp).clip(CircleShape).background(accentColor.copy(alpha = 0.15f)),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(imageVector = icon, contentDescription = null, tint = accentColor, modifier = Modifier.size(AmitiaIconSize.Medium))
                }
                Column(modifier = Modifier.weight(1f)) {
                    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
                        Text(
                            text = rule.name,
                            style = MaterialTheme.typography.titleSmall,
                            color = MaterialTheme.colorScheme.onSurface,
                            fontWeight = FontWeight.Medium,
                            maxLines = 1, overflow = TextOverflow.Ellipsis
                        )
                        Surface(shape = MaterialTheme.shapes.small, color = MaterialTheme.colorScheme.surfaceVariant) {
                            Text(
                                text = rule.type.label,
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                                modifier = Modifier.padding(horizontal = 4.dp, vertical = 1.dp)
                            )
                        }
                    }
                    Text(
                        text = rule.description,
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        maxLines = 2, overflow = TextOverflow.Ellipsis
                    )
                }
            }
            if (rule.config.isNotEmpty()) {
                Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
                    rule.config.forEach { (key, value) ->
                        Text(
                            text = "$key：$value",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.7f)
                        )
                    }
                }
            }
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                AmitiaIconButton(
                    icon = AmitiaIcons.ArrowUpward,
                    contentDescription = "上移",
                    onClick = onMoveUp,
                    enabled = !isFirst,
                    tint = if (!isFirst) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.3f)
                )
                AmitiaIconButton(
                    icon = AmitiaIcons.ArrowDownward,
                    contentDescription = "下移",
                    onClick = onMoveDown,
                    enabled = !isLast,
                    tint = if (!isLast) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.3f)
                )
                Box(modifier = Modifier.weight(1f))
                AmitiaSwitchRow(
                    title = if (rule.enabled) "已启用" else "已禁用",
                    checked = rule.enabled,
                    onCheckedChange = onToggle
                )
            }
        }
    }
}

@Composable
private fun ruleIconAndColor(type: SafetyRuleType): Pair<ImageVector, androidx.compose.ui.graphics.Color> {
    return when (type) {
        SafetyRuleType.BlockedApp -> AmitiaIcons.Block to MaterialTheme.colorScheme.error
        SafetyRuleType.BlockedOperation -> AmitiaIcons.Block to MaterialTheme.colorScheme.error
        SafetyRuleType.PaymentProtection -> AmitiaIcons.Gavel to MaterialTheme.colorScheme.error
        SafetyRuleType.PrivacyInput -> AmitiaIcons.Lock to MaterialTheme.colorScheme.tertiary
        SafetyRuleType.NightRestriction -> AmitiaIcons.Bedtime to MaterialTheme.colorScheme.secondary
        SafetyRuleType.AutoStop -> AmitiaIcons.Stop to MaterialTheme.colorScheme.primary
    }
}

@Preview(name = "Safety Rules - Light", showBackground = true)
@Composable
private fun SafetyRulesLightPreview() {
    AmitiaTheme(darkTheme = false) {
        SafetyRulesContent(
            rules = listOf(
                SafetyRule("r1", "禁止操作支付应用", "拦截支付宝、微信支付等金融操作", true, 1, SafetyRuleType.PaymentProtection),
                SafetyRule("r2", "禁止读取密码输入", "在密码输入框时不执行任何操作", true, 2, SafetyRuleType.PrivacyInput),
                SafetyRule("r3", "夜间限制", "23:00 - 07:00 禁止 Computer Use", false, 3, SafetyRuleType.NightRestriction, mapOf("开始" to "23:00", "结束" to "07:00"))
            ),
            loading = false, onBack = {}, onToggle = { _, _ -> }, onMoveUp = {}, onMoveDown = {}
        )
    }
}

@Preview(name = "Safety Rules - Dark", showBackground = true)
@Composable
private fun SafetyRulesDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        SafetyRulesContent(
            rules = emptyList(), loading = true, onBack = {},
            onToggle = { _, _ -> }, onMoveUp = {}, onMoveDown = {}
        )
    }
}
