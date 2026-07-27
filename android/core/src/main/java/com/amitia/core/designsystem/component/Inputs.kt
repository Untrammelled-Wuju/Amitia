package com.amitia.core.designsystem.component

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
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Slider
import androidx.compose.material3.SliderDefaults
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextField
import androidx.compose.material3.TextFieldDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaInputShape
import com.amitia.core.designsystem.AmitiaListItemShape
import com.amitia.core.designsystem.AmitiaPillShape
import com.amitia.core.designsystem.AmitiaRadius
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaTouchTarget

@Composable
fun AmitiaTextField(
    value: String,
    onValueChange: (String) -> Unit,
    modifier: Modifier = Modifier,
    label: String? = null,
    placeholder: String? = null,
    leadingIcon: androidx.compose.ui.graphics.vector.ImageVector? = null,
    trailingIcon: androidx.compose.ui.graphics.vector.ImageVector? = null,
    onTrailingClick: (() -> Unit)? = null,
    enabled: Boolean = true,
    isError: Boolean = false,
    errorMessage: String? = null,
    singleLine: Boolean = true
) {
    OutlinedTextField(
        value = value,
        onValueChange = onValueChange,
        modifier = modifier.fillMaxWidth(),
        label = label?.let {
            { Text(text = it, style = MaterialTheme.typography.bodyMedium) }
        },
        placeholder = placeholder?.let {
            { Text(text = it, style = MaterialTheme.typography.bodyMedium) }
        },
        leadingIcon = leadingIcon?.let {
            { Icon(imageVector = it, contentDescription = null, modifier = Modifier.size(AmitiaIconSize.Medium)) }
        },
        trailingIcon = {
            if (trailingIcon != null) {
                AmitiaIconButton(
                    icon = trailingIcon,
                    contentDescription = null,
                    onClick = onTrailingClick ?: {},
                    modifier = Modifier.size(AmitiaTouchTarget.Minimum)
                )
            }
        },
        enabled = enabled,
        isError = isError,
        supportingText = errorMessage?.let {
            { Text(text = it, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.error) }
        },
        singleLine = singleLine,
        shape = AmitiaInputShape,
        textStyle = MaterialTheme.typography.bodyMedium.copy(color = MaterialTheme.colorScheme.onSurface),
        colors = OutlinedTextFieldDefaults.colors(
            focusedBorderColor = MaterialTheme.colorScheme.primary,
            unfocusedBorderColor = MaterialTheme.colorScheme.outline,
            errorBorderColor = MaterialTheme.colorScheme.error,
            disabledBorderColor = MaterialTheme.colorScheme.outline.copy(alpha = 0.38f),
            focusedContainerColor = MaterialTheme.colorScheme.surface,
            unfocusedContainerColor = MaterialTheme.colorScheme.surface,
            disabledContainerColor = MaterialTheme.colorScheme.surface.copy(alpha = 0.38f),
            focusedLabelColor = MaterialTheme.colorScheme.primary,
            unfocusedLabelColor = MaterialTheme.colorScheme.onSurfaceVariant,
            errorLabelColor = MaterialTheme.colorScheme.error,
            focusedLeadingIconColor = MaterialTheme.colorScheme.primary,
            unfocusedLeadingIconColor = MaterialTheme.colorScheme.onSurfaceVariant,
            focusedTrailingIconColor = MaterialTheme.colorScheme.primary,
            unfocusedTrailingIconColor = MaterialTheme.colorScheme.onSurfaceVariant,
            cursorColor = MaterialTheme.colorScheme.primary
        )
    )
}

@Composable
fun AmitiaSearchField(
    value: String,
    onValueChange: (String) -> Unit,
    modifier: Modifier = Modifier,
    placeholder: String = "搜索",
    onClear: (() -> Unit)? = null,
    enabled: Boolean = true
) {
    val interactionSource = remember { MutableInteractionSource() }
    Surface(
        modifier = modifier
            .fillMaxWidth()
            .height(AmitiaTouchTarget.Minimum),
        shape = AmitiaPillShape,
        color = MaterialTheme.colorScheme.surfaceVariant
    ) {
        Row(
            modifier = Modifier
                .fillMaxSize()
                .padding(horizontal = AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Icon(
                imageVector = AmitiaIcons.Search,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.size(AmitiaIconSize.Medium)
            )
            TextField(
                value = value,
                onValueChange = onValueChange,
                modifier = Modifier.weight(1f),
                placeholder = {
                    Text(
                        text = placeholder,
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                },
                singleLine = true,
                enabled = enabled,
                textStyle = MaterialTheme.typography.bodyMedium.copy(color = MaterialTheme.colorScheme.onSurface),
                colors = TextFieldDefaults.colors(
                    focusedContainerColor = Color.Transparent,
                    unfocusedContainerColor = Color.Transparent,
                    disabledContainerColor = Color.Transparent,
                    focusedIndicatorColor = Color.Transparent,
                    unfocusedIndicatorColor = Color.Transparent,
                    disabledIndicatorColor = Color.Transparent,
                    cursorColor = MaterialTheme.colorScheme.primary
                )
            )
            if (value.isNotEmpty() && onClear != null) {
                AmitiaIconButton(
                    icon = AmitiaIcons.Close,
                    contentDescription = "清除",
                    onClick = onClear,
                    modifier = Modifier.size(AmitiaTouchTarget.Minimum)
                )
            }
        }
    }
}

@Composable
fun AmitiaPasswordField(
    value: String,
    onValueChange: (String) -> Unit,
    modifier: Modifier = Modifier,
    label: String? = null,
    placeholder: String? = null,
    enabled: Boolean = true,
    isError: Boolean = false,
    errorMessage: String? = null
) {
    var passwordVisible by remember { mutableStateOf(false) }
    OutlinedTextField(
        value = value,
        onValueChange = onValueChange,
        modifier = modifier.fillMaxWidth(),
        label = label?.let {
            { Text(text = it, style = MaterialTheme.typography.bodyMedium) }
        },
        placeholder = placeholder?.let {
            { Text(text = it, style = MaterialTheme.typography.bodyMedium) }
        },
        leadingIcon = {
            Icon(
                imageVector = AmitiaIcons.Lock,
                contentDescription = null,
                modifier = Modifier.size(AmitiaIconSize.Medium),
                tint = MaterialTheme.colorScheme.onSurfaceVariant
            )
        },
        trailingIcon = {
            AmitiaIconButton(
                icon = if (passwordVisible) AmitiaIcons.VisibilityOff else AmitiaIcons.Visibility,
                contentDescription = if (passwordVisible) "隐藏密码" else "显示密码",
                onClick = { passwordVisible = !passwordVisible },
                modifier = Modifier.size(AmitiaTouchTarget.Minimum)
            )
        },
        enabled = enabled,
        isError = isError,
        supportingText = errorMessage?.let {
            { Text(text = it, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.error) }
        },
        singleLine = true,
        visualTransformation = if (passwordVisible) VisualTransformation.None else PasswordVisualTransformation(),
        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password),
        shape = AmitiaInputShape,
        textStyle = MaterialTheme.typography.bodyMedium.copy(color = MaterialTheme.colorScheme.onSurface),
        colors = OutlinedTextFieldDefaults.colors(
            focusedBorderColor = MaterialTheme.colorScheme.primary,
            unfocusedBorderColor = MaterialTheme.colorScheme.outline,
            errorBorderColor = MaterialTheme.colorScheme.error,
            disabledBorderColor = MaterialTheme.colorScheme.outline.copy(alpha = 0.38f),
            focusedContainerColor = MaterialTheme.colorScheme.surface,
            unfocusedContainerColor = MaterialTheme.colorScheme.surface,
            disabledContainerColor = MaterialTheme.colorScheme.surface.copy(alpha = 0.38f),
            cursorColor = MaterialTheme.colorScheme.primary
        )
    )
}

@Composable
fun AmitiaMultilineField(
    value: String,
    onValueChange: (String) -> Unit,
    modifier: Modifier = Modifier,
    label: String? = null,
    placeholder: String? = null,
    enabled: Boolean = true,
    minLines: Int = 3,
    maxLines: Int = 6,
    charLimit: Int? = null
) {
    Column(modifier = modifier.fillMaxWidth()) {
        OutlinedTextField(
            value = value,
            onValueChange = { newValue ->
                if (charLimit == null || newValue.length <= charLimit) {
                    onValueChange(newValue)
                }
            },
            modifier = Modifier.fillMaxWidth(),
            label = label?.let {
                { Text(text = it, style = MaterialTheme.typography.bodyMedium) }
            },
            placeholder = placeholder?.let {
                { Text(text = it, style = MaterialTheme.typography.bodyMedium) }
            },
            enabled = enabled,
            minLines = minLines,
            maxLines = maxLines,
            shape = AmitiaInputShape,
            textStyle = MaterialTheme.typography.bodyMedium.copy(color = MaterialTheme.colorScheme.onSurface),
            colors = OutlinedTextFieldDefaults.colors(
                focusedBorderColor = MaterialTheme.colorScheme.primary,
                unfocusedBorderColor = MaterialTheme.colorScheme.outline,
                disabledBorderColor = MaterialTheme.colorScheme.outline.copy(alpha = 0.38f),
                focusedContainerColor = MaterialTheme.colorScheme.surface,
                unfocusedContainerColor = MaterialTheme.colorScheme.surface,
                disabledContainerColor = MaterialTheme.colorScheme.surface.copy(alpha = 0.38f),
                cursorColor = MaterialTheme.colorScheme.primary
            )
        )
        if (charLimit != null) {
            Text(
                text = "${value.length} / $charLimit",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier
                    .padding(top = AmitiaSpacing.Xs)
                    .align(Alignment.End)
            )
        }
    }
}

@Composable
fun AmitiaNumberField(
    value: String,
    onValueChange: (String) -> Unit,
    modifier: Modifier = Modifier,
    label: String? = null,
    placeholder: String? = null,
    enabled: Boolean = true,
    onIncrement: (() -> Unit)? = null,
    onDecrement: (() -> Unit)? = null,
    unit: String? = null
) {
    OutlinedTextField(
        value = value,
        onValueChange = onValueChange,
        modifier = modifier.fillMaxWidth(),
        label = label?.let {
            { Text(text = it, style = MaterialTheme.typography.bodyMedium) }
        },
        placeholder = placeholder?.let {
            { Text(text = it, style = MaterialTheme.typography.bodyMedium) }
        },
        enabled = enabled,
        singleLine = true,
        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
        leadingIcon = onDecrement?.let {
            {
                AmitiaIconButton(
                    icon = AmitiaIcons.Remove,
                    contentDescription = "减少",
                    onClick = it,
                    modifier = Modifier.size(AmitiaTouchTarget.Minimum)
                )
            }
        },
        trailingIcon = {
            Row(verticalAlignment = Alignment.CenterVertically) {
                if (unit != null) {
                    Text(
                        text = unit,
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                onIncrement?.let {
                    AmitiaIconButton(
                        icon = AmitiaIcons.Add,
                        contentDescription = "增加",
                        onClick = it,
                        modifier = Modifier.size(AmitiaTouchTarget.Minimum)
                    )
                }
            }
        },
        shape = AmitiaInputShape,
        textStyle = MaterialTheme.typography.bodyMedium.copy(color = MaterialTheme.colorScheme.onSurface),
        colors = OutlinedTextFieldDefaults.colors(
            focusedBorderColor = MaterialTheme.colorScheme.primary,
            unfocusedBorderColor = MaterialTheme.colorScheme.outline,
            disabledBorderColor = MaterialTheme.colorScheme.outline.copy(alpha = 0.38f),
            focusedContainerColor = MaterialTheme.colorScheme.surface,
            unfocusedContainerColor = MaterialTheme.colorScheme.surface,
            disabledContainerColor = MaterialTheme.colorScheme.surface.copy(alpha = 0.38f),
            cursorColor = MaterialTheme.colorScheme.primary
        )
    )
}

@Composable
fun AmitiaSlider(
    value: Float,
    onValueChange: (Float) -> Unit,
    modifier: Modifier = Modifier,
    valueRange: ClosedFloatingPointRange<Float> = 0f..1f,
    steps: Int = 0,
    enabled: Boolean = true,
    label: String? = null,
    valueFormatter: ((Float) -> String)? = null
) {
    Column(modifier = modifier.fillMaxWidth()) {
        if (label != null || valueFormatter != null) {
            Row(
                modifier = Modifier.fillMaxWidth().padding(bottom = AmitiaSpacing.Xs),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                if (label != null) {
                    Text(
                        text = label,
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                if (valueFormatter != null) {
                    Text(
                        text = valueFormatter(value),
                        style = MaterialTheme.typography.labelLarge,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                }
            }
        }
        Slider(
            value = value,
            onValueChange = onValueChange,
            modifier = Modifier.fillMaxWidth(),
            enabled = enabled,
            valueRange = valueRange,
            steps = steps,
            colors = SliderDefaults.colors(
                thumbColor = MaterialTheme.colorScheme.primary,
                activeTrackColor = MaterialTheme.colorScheme.primary,
                inactiveTrackColor = MaterialTheme.colorScheme.surfaceVariant,
                activeTickColor = MaterialTheme.colorScheme.primary,
                inactiveTickColor = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.38f),
                disabledThumbColor = MaterialTheme.colorScheme.primary.copy(alpha = 0.38f),
                disabledActiveTrackColor = MaterialTheme.colorScheme.primary.copy(alpha = 0.38f),
                disabledInactiveTrackColor = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.38f)
            )
        )
    }
}

@Composable
fun AmitiaSwitchRow(
    title: String,
    checked: Boolean,
    onCheckedChange: (Boolean) -> Unit,
    modifier: Modifier = Modifier,
    subtitle: String? = null,
    enabled: Boolean = true,
    leadingIcon: androidx.compose.ui.graphics.vector.ImageVector? = null
) {
    val interactionSource = remember { MutableInteractionSource() }
    Row(
        modifier = modifier
            .fillMaxWidth()
            .heightIn(min = AmitiaTouchTarget.Minimum)
            .clip(AmitiaListItemShape)
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                enabled = enabled,
                role = Role.Switch,
                onClick = { onCheckedChange(!checked) }
            )
            .padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
    ) {
        if (leadingIcon != null) {
            Box(
                modifier = Modifier
                    .size(AmitiaIconSize.Large)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.primaryContainer),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = leadingIcon,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onPrimaryContainer,
                    modifier = Modifier.size(AmitiaIconSize.Medium)
                )
            }
        }
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = title,
                style = MaterialTheme.typography.bodyLarge,
                color = if (enabled) MaterialTheme.colorScheme.onSurface else MaterialTheme.colorScheme.onSurface.copy(alpha = 0.38f),
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
            if (subtitle != null) {
                Text(
                    text = subtitle,
                    style = MaterialTheme.typography.bodySmall,
                    color = if (enabled) MaterialTheme.colorScheme.onSurfaceVariant else MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.38f),
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis
                )
            }
        }
        Switch(
            checked = checked,
            onCheckedChange = onCheckedChange,
            enabled = enabled,
            colors = SwitchDefaults.colors(
                checkedThumbColor = MaterialTheme.colorScheme.onPrimary,
                checkedTrackColor = MaterialTheme.colorScheme.primary,
                uncheckedThumbColor = MaterialTheme.colorScheme.surface,
                uncheckedTrackColor = MaterialTheme.colorScheme.surfaceVariant,
                disabledCheckedThumbColor = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.38f),
                disabledUncheckedThumbColor = MaterialTheme.colorScheme.surface.copy(alpha = 0.38f)
            )
        )
    }
}

@Composable
fun AmitiaSelectionRow(
    title: String,
    selected: Boolean,
    onSelect: () -> Unit,
    modifier: Modifier = Modifier,
    subtitle: String? = null,
    multiSelect: Boolean = false,
    enabled: Boolean = true,
    leadingIcon: androidx.compose.ui.graphics.vector.ImageVector? = null
) {
    val interactionSource = remember { MutableInteractionSource() }
    Row(
        modifier = modifier
            .fillMaxWidth()
            .heightIn(min = AmitiaTouchTarget.Minimum)
            .clip(AmitiaListItemShape)
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                enabled = enabled,
                role = if (multiSelect) Role.Checkbox else Role.RadioButton,
                onClick = onSelect
            )
            .padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
    ) {
        if (leadingIcon != null) {
            Box(
                modifier = Modifier
                    .size(AmitiaIconSize.Large)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.primaryContainer),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = leadingIcon,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onPrimaryContainer,
                    modifier = Modifier.size(AmitiaIconSize.Medium)
                )
            }
        }
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = title,
                style = MaterialTheme.typography.bodyLarge,
                color = if (enabled) MaterialTheme.colorScheme.onSurface else MaterialTheme.colorScheme.onSurface.copy(alpha = 0.38f),
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
            if (subtitle != null) {
                Text(
                    text = subtitle,
                    style = MaterialTheme.typography.bodySmall,
                    color = if (enabled) MaterialTheme.colorScheme.onSurfaceVariant else MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.38f),
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis
                )
            }
        }
        Icon(
            imageVector = if (multiSelect) {
                if (selected) AmitiaIcons.CheckBox else AmitiaIcons.CheckBoxOutlineBlank
            } else {
                if (selected) AmitiaIcons.RadioButtonChecked else AmitiaIcons.RadioButtonUnchecked
            },
            contentDescription = null,
            tint = if (selected && enabled) MaterialTheme.colorScheme.primary
            else MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = if (enabled) 1f else 0.38f),
            modifier = Modifier.size(AmitiaIconSize.Medium)
        )
    }
}

data class AmitiaChipItem(
    val label: String,
    val selected: Boolean = false
)

@Composable
fun AmitiaChipSelector(
    items: List<AmitiaChipItem>,
    onToggle: (Int) -> Unit,
    modifier: Modifier = Modifier,
    multiSelect: Boolean = true
) {
    LazyRow(
        modifier = modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        items(items.size) { index ->
            val item = items[index]
            val interactionSource = remember { MutableInteractionSource() }
            Surface(
                modifier = Modifier
                    .clip(AmitiaPillShape)
                    .clickable(
                        interactionSource = interactionSource,
                        indication = null,
                        role = if (multiSelect) Role.Checkbox else Role.RadioButton,
                        onClick = { onToggle(index) }
                    ),
                shape = AmitiaPillShape,
                color = if (item.selected) MaterialTheme.colorScheme.primaryContainer
                else MaterialTheme.colorScheme.surfaceVariant
            ) {
                Text(
                    text = item.label,
                    style = MaterialTheme.typography.labelMedium,
                    color = if (item.selected) MaterialTheme.colorScheme.onPrimaryContainer
                    else MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm)
                )
            }
        }
    }
}

@Composable
fun AmitiaCodeEditorSurface(
    value: String,
    onValueChange: (String) -> Unit,
    modifier: Modifier = Modifier,
    language: String = "json",
    enabled: Boolean = true
) {
    Column(
        modifier = modifier
            .fillMaxWidth()
            .clip(AmitiaInputShape)
            .background(MaterialTheme.colorScheme.surfaceVariant)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Icon(
                    imageVector = AmitiaIcons.Code,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.size(AmitiaIconSize.Medium)
                )
                Text(
                    text = language.uppercase(),
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            Text(
                text = "${value.length} chars",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
            )
        }
        OutlinedTextField(
            value = value,
            onValueChange = onValueChange,
            modifier = Modifier.fillMaxWidth(),
            enabled = enabled,
            textStyle = MaterialTheme.typography.bodySmall.copy(
                color = MaterialTheme.colorScheme.onSurface,
                fontFamily = FontFamily.Monospace
            ),
            colors = TextFieldDefaults.colors(
                focusedContainerColor = Color.Transparent,
                unfocusedContainerColor = Color.Transparent,
                disabledContainerColor = Color.Transparent,
                focusedIndicatorColor = Color.Transparent,
                unfocusedIndicatorColor = Color.Transparent,
                disabledIndicatorColor = Color.Transparent,
                cursorColor = MaterialTheme.colorScheme.primary
            )
        )
    }
}

@Preview(name = "Inputs - Light", showBackground = true)
@Composable
private fun AmitiaInputsLightPreview() {
    var text by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var multiline by remember { mutableStateOf("") }
    var code by remember { mutableStateOf("{\n  \"key\": \"value\"\n}") }
    var sliderValue by remember { mutableStateOf(0.5f) }
    var switchChecked by remember { mutableStateOf(true) }
    var selectedIndex by remember { mutableStateOf(0) }
    val chips = remember {
        mutableStateOf(
            listOf(
                AmitiaChipItem("标签一", true),
                AmitiaChipItem("标签二", false),
                AmitiaChipItem("标签三", true)
            )
        )
    }
    AmitiaTheme(darkTheme = false) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            AmitiaTextField(
                value = text,
                onValueChange = { text = it },
                label = "用户名",
                placeholder = "请输入用户名"
            )
            AmitiaSearchField(
                value = text,
                onValueChange = { text = it },
                onClear = { text = "" }
            )
            AmitiaPasswordField(
                value = password,
                onValueChange = { password = it },
                label = "密码"
            )
            AmitiaMultilineField(
                value = multiline,
                onValueChange = { multiline = it },
                label = "描述",
                charLimit = 200
            )
            AmitiaNumberField(
                value = "100",
                onValueChange = {},
                label = "温度",
                unit = "°C",
                onIncrement = {},
                onDecrement = {}
            )
            AmitiaSlider(
                value = sliderValue,
                onValueChange = { sliderValue = it },
                label = "音量",
                valueFormatter = { "${(it * 100).toInt()}%" }
            )
            AmitiaSwitchRow(
                title = "启用通知",
                subtitle = "接收消息推送提醒",
                checked = switchChecked,
                onCheckedChange = { switchChecked = it },
                leadingIcon = AmitiaIcons.Notifications
            )
            AmitiaSelectionRow(
                title = "选项一",
                subtitle = "这是选项一的描述",
                selected = selectedIndex == 0,
                onSelect = { selectedIndex = 0 },
                leadingIcon = AmitiaIcons.StarBorder
            )
            AmitiaChipSelector(
                items = chips.value,
                onToggle = { index ->
                    chips.value = chips.value.mapIndexed { i, item ->
                        if (i == index) item.copy(selected = !item.selected) else item
                    }
                }
            )
            AmitiaCodeEditorSurface(
                value = code,
                onValueChange = { code = it },
                language = "json"
            )
        }
    }
}

@Preview(name = "Inputs - Dark", showBackground = true)
@Composable
private fun AmitiaInputsDarkPreview() {
    var text by remember { mutableStateOf("测试文本") }
    AmitiaTheme(darkTheme = true) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            AmitiaTextField(
                value = text,
                onValueChange = { text = it },
                label = "用户名"
            )
            AmitiaSwitchRow(
                title = "暗色开关",
                checked = true,
                onCheckedChange = {}
            )
        }
    }
}
