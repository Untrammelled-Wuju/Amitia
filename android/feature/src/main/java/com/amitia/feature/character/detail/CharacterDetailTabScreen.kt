package com.amitia.feature.character.detail

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Edit
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.ScrollableTabRow
import androidx.compose.material3.Tab
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.component.AmitiaLoadingIndicator
import com.amitia.feature.character.CharacterDetailViewModel
import kotlinx.coroutines.launch

data class CharacterDetailTab(
    val title: String,
    val content: @Composable (PaddingValues) -> Unit
)

@Composable
fun CharacterDetailTabScreen(
    characterId: String,
    onEdit: () -> Unit,
    onBack: () -> Unit,
    onChat: () -> Unit,
    viewModel: CharacterDetailViewModel = hiltViewModel()
) {
    LaunchedEffect(characterId) {
        viewModel.loadAll(characterId)
    }

    val overviewState by viewModel.overviewState.collectAsStateWithLifecycle()

    val tabs = listOf(
        CharacterDetailTab("概览") { CharacterOverviewTab(characterId, viewModel, onChat, it) },
        CharacterDetailTab("形象") { CharacterAppearanceTab(characterId, viewModel, it) },
        CharacterDetailTab("基础") { CharacterBasicSettingsTab(characterId, viewModel, it) },
        CharacterDetailTab("性格") { CharacterPersonalityTab(viewModel, it) },
        CharacterDetailTab("关系") { CharacterRelationshipTab(viewModel, it) },
        CharacterDetailTab("情绪") { CharacterEmotionTab(viewModel, it) },
        CharacterDetailTab("生活") { CharacterLifeStatusTab(viewModel, it) },
        CharacterDetailTab("主动消息") { CharacterProactiveTab(viewModel, it) },
        CharacterDetailTab("声音") { CharacterVoiceTab(viewModel, it) },
        CharacterDetailTab("模型") { CharacterModelBindingTab(viewModel, it) },
        CharacterDetailTab("记忆") { CharacterMemoryTab(viewModel, it) },
        CharacterDetailTab("渠道") { CharacterChannelTab(viewModel, it) },
        CharacterDetailTab("能力") { CharacterCapabilityTab(viewModel, it) },
        CharacterDetailTab("权限") { CharacterPermissionTab(viewModel, it) },
        CharacterDetailTab("数据") { CharacterDataTab(viewModel, it) },
        CharacterDetailTab("归档") { CharacterArchiveTab(viewModel, it) }
    )

    val pagerState = rememberPagerState(pageCount = { tabs.size })
    val scope = rememberCoroutineScope()

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = {
                    val name = (overviewState as? com.amitia.core.designsystem.ScreenState.Content)?.data?.name
                    Text(
                        text = name ?: "角色详情",
                        style = MaterialTheme.typography.titleLarge,
                        fontWeight = FontWeight.Medium,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                },
                actions = {
                    IconButton(onClick = onEdit) {
                        Icon(Icons.Outlined.Edit, contentDescription = "编辑")
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background,
                    titleContentColor = MaterialTheme.colorScheme.onBackground,
                    actionIconContentColor = MaterialTheme.colorScheme.onSurfaceVariant
                )
            )
        }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
        ) {
            ScrollableTabRow(
                selectedTabIndex = pagerState.currentPage,
                edgePadding = 0.dp,
                containerColor = MaterialTheme.colorScheme.background,
                contentColor = MaterialTheme.colorScheme.primary,
                divider = {}
            ) {
                tabs.forEachIndexed { index, tab ->
                    Tab(
                        selected = pagerState.currentPage == index,
                        onClick = { scope.launch { pagerState.animateScrollToPage(index) } },
                        text = {
                            Text(
                                text = tab.title,
                                style = MaterialTheme.typography.labelMedium,
                                color = if (pagerState.currentPage == index)
                                    MaterialTheme.colorScheme.primary
                                else MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }
                    )
                }
            }
            HorizontalPager(
                state = pagerState,
                modifier = Modifier.fillMaxSize()
            ) { page ->
                tabs[page].content(PaddingValues(0.dp))
            }
        }
    }
}

@Composable
fun DetailLoadingBox(modifier: Modifier = Modifier) {
    Box(
        modifier = modifier.fillMaxSize(),
        contentAlignment = Alignment.Center
    ) {
        AmitiaLoadingIndicator()
    }
}

@Composable
fun DetailEmptyBox(
    message: String,
    modifier: Modifier = Modifier
) {
    Box(
        modifier = modifier.fillMaxSize(),
        contentAlignment = Alignment.Center
    ) {
        Text(
            text = message,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
    }
}

@Composable
fun DetailErrorBox(
    message: String,
    onRetry: () -> Unit,
    modifier: Modifier = Modifier
) {
    com.amitia.core.designsystem.component.AmitiaErrorState(
        icon = com.amitia.core.designsystem.AmitiaIcons.Error,
        title = "加载失败",
        description = message,
        onRetry = onRetry,
        modifier = modifier.fillMaxSize()
    )
}
