package com.amitia.feature.creativeworkshop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
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
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
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
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaMultilineField
import com.amitia.core.designsystem.component.AmitiaSelectionRow
import com.amitia.core.designsystem.component.AmitiaTextField
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun NewProjectScreen(
    onBack: () -> Unit,
    onCreate: (CwProjectType, String, String) -> Unit
) {
    var selectedType by remember { mutableStateOf(CwProjectType.Skill) }
    var projectName by remember { mutableStateOf("") }
    var projectDesc by remember { mutableStateOf("") }
    val canCreate = projectName.isNotBlank()

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = { Text("新建扩展项目", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Medium) },
                navigationIcon = { AmitiaIconButton(icon = AmitiaIcons.ArrowBack, contentDescription = "返回", onClick = onBack) },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background,
                    titleContentColor = MaterialTheme.colorScheme.onBackground
                )
            )
        },
        bottomBar = {
            Surface(color = MaterialTheme.colorScheme.background) {
                PrimaryButton(
                    text = "创建项目",
                    onClick = { onCreate(selectedType, projectName, projectDesc) },
                    enabled = canCreate,
                    leadingIcon = AmitiaIcons.Add,
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(AmitiaSpacing.Base)
                )
            }
        }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .verticalScroll(rememberScrollState())
                .padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            AmitiaSectionHeader(title = "选择项目类型")
            CwProjectType.entries.forEach { type ->
                Surface(
                    modifier = Modifier.fillMaxWidth(),
                    shape = MaterialTheme.shapes.large,
                    color = if (selectedType == type) MaterialTheme.colorScheme.primaryContainer
                    else MaterialTheme.colorScheme.surface,
                    onClick = { selectedType = type }
                ) {
                    Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
                        AmitiaSelectionRow(
                            title = type.label,
                            subtitle = type.description,
                            selected = selectedType == type,
                            onSelect = { selectedType = type },
                            leadingIcon = type.icon()
                        )
                    }
                }
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            AmitiaSectionHeader(title = "项目信息")
            AmitiaTextField(
                value = projectName,
                onValueChange = { projectName = it },
                label = "项目名称",
                placeholder = "输入项目名称",
                leadingIcon = AmitiaIcons.Edit
            )
            AmitiaMultilineField(
                value = projectDesc,
                onValueChange = { projectDesc = it },
                label = "项目描述",
                placeholder = "描述项目用途和功能",
                charLimit = 500
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }
}

@Preview(name = "New Project - Light", showBackground = true)
@Composable
private fun NewProjectScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        NewProjectScreen(onBack = {}, onCreate = { _, _, _ -> })
    }
}

@Preview(name = "New Project - Dark", showBackground = true)
@Composable
private fun NewProjectScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        NewProjectScreen(onBack = {}, onCreate = { _, _, _ -> })
    }
}
