package com.amitia.feature.settings.licenses

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
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
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.AmitiaContentSurface
import com.amitia.core.designsystem.component.AmitiaPageScaffold
import com.amitia.core.designsystem.component.AmitiaSearchField
import com.amitia.core.designsystem.component.AmitiaInsetDivider
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.feature.settings.LicenseItem
import com.amitia.feature.settings.SettingsCenterViewModel

@Composable
fun OpenSourceLicensesScreen(
    onBack: () -> Unit,
    viewModel: SettingsCenterViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val licenses = state.licenses
    var searchQuery by remember { mutableStateOf("") }

    OpenSourceLicensesScreenContent(
        licenses = licenses,
        searchQuery = searchQuery,
        onSearchChange = { searchQuery = it },
        onBack = onBack
    )
}

@Composable
private fun OpenSourceLicensesScreenContent(
    licenses: List<LicenseItem>,
    searchQuery: String,
    onSearchChange: (String) -> Unit,
    onBack: () -> Unit
) {
    val filtered = licenses.filter {
        it.name.contains(searchQuery, ignoreCase = true) ||
        it.license.contains(searchQuery, ignoreCase = true)
    }

    AmitiaPageScaffold(
        topBar = { AmitiaTopBar(title = "开源许可", onBack = onBack) }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
        ) {
            AmitiaSearchField(
                value = searchQuery,
                onValueChange = onSearchChange,
                onClear = { onSearchChange("") },
                placeholder = "搜索依赖或许可证",
                modifier = Modifier.padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm)
            )
            if (filtered.isEmpty()) {
                AmitiaEmptyState(
                    icon = AmitiaIcons.Search,
                    title = "未找到结果",
                    description = "尝试使用其他关键词搜索"
                )
            } else {
                LazyColumn(
                    modifier = Modifier.fillMaxSize(),
                    contentPadding = androidx.compose.foundation.layout.PaddingValues(
                        horizontal = AmitiaSpacing.Base,
                        vertical = AmitiaSpacing.Sm
                    ),
                    verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    items(filtered) { license ->
                        LicenseItemRow(license = license)
                    }
                }
            }
        }
    }
}

@Composable
private fun LicenseItemRow(license: LicenseItem) {
    AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = license.name,
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = "v${license.version}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                Text(
                    text = license.license,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.primary,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
            }
        }
    }
}

@Preview(name = "开源许可页 - 亮色", showBackground = true)
@Composable
private fun OpenSourceLicensesScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        OpenSourceLicensesScreenContent(
            licenses = listOf(
                LicenseItem("Jetpack Compose", "1.7", "Apache 2.0", ""),
                LicenseItem("Kotlin", "2.0", "Apache 2.0", ""),
                LicenseItem("Hilt", "2.51", "Apache 2.0", "")
            ),
            searchQuery = "",
            onSearchChange = {},
            onBack = {}
        )
    }
}

@Preview(name = "开源许可页 - 暗色", showBackground = true)
@Composable
private fun OpenSourceLicensesScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        OpenSourceLicensesScreenContent(
            licenses = listOf(
                LicenseItem("Jetpack Compose", "1.7", "Apache 2.0", "")
            ),
            searchQuery = "jetpack",
            onSearchChange = {},
            onBack = {}
        )
    }
}
