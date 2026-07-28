package com.amitia.android.navigation

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.navigation.NavDestination.Companion.hierarchy
import androidx.navigation.NavHostController
import androidx.navigation.compose.currentBackStackEntryAsState
import com.amitia.core.designsystem.AmitiaGlassSurface
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaRadius
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.GlassLevel

data class AmitiaDrawerItem(
    val route: String,
    val label: String,
    val subtitle: String,
    val icon: ImageVector
)

data class AmitiaDrawerSection(
    val title: String,
    val items: List<AmitiaDrawerItem>
)

val amitiaDrawerSections: List<AmitiaDrawerSection> = listOf(
    AmitiaDrawerSection(
        title = "对话与陪伴",
        items = listOf(
            AmitiaDrawerItem(AmitiaRoutes.Main.CHAT, "当前对话", "继续与 Amitia 的聊天", AmitiaIcons.Chat),
            AmitiaDrawerItem(AmitiaRoutes.Main.CHAT_CONVERSATION, "会话列表", "查看全部角色会话", AmitiaIcons.GridView),
            AmitiaDrawerItem(AmitiaRoutes.Main.CHARACTER, "角色", "角色、设定与生活状态", AmitiaIcons.Person),
            AmitiaDrawerItem(AmitiaRoutes.Main.MEMORY, "记忆", "长期记忆与关系时间线", AmitiaIcons.Psychology)
        )
    ),
    AmitiaDrawerSection(
        title = "工具与系统",
        items = listOf(
            AmitiaDrawerItem(AmitiaRoutes.Main.SCHEDULE, "日程与提醒", "计划、提醒与主动消息", AmitiaIcons.CalendarMonth),
            AmitiaDrawerItem(AmitiaRoutes.Main.CHANNEL_CENTER, "渠道中心", "微信、QQ、Web 与消息连接", AmitiaIcons.Hub),
            AmitiaDrawerItem(AmitiaRoutes.Main.EXTENSION_CENTER, "扩展中心", "Skill、Plugin 与 MCP", AmitiaIcons.Extension),
            AmitiaDrawerItem(AmitiaRoutes.Main.MORE, "更多与设置", "模型、渠道、数据与系统", AmitiaIcons.Settings)
        )
    )
)

@Composable
fun AmitiaAppDrawer(
    navController: NavHostController,
    isOpen: Boolean,
    onOpen: () -> Unit,
    onClose: () -> Unit,
    onOpenProfile: () -> Unit
) {
    val backStackEntry by navController.currentBackStackEntryAsState()
    val currentDestination = backStackEntry?.destination

    Box(modifier = Modifier.fillMaxSize()) {
        AnimatedVisibility(
            visible = isOpen,
            enter = fadeIn(),
            exit = fadeOut()
        ) {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .background(Color(0x59080B09))
                    .clickable(onClick = onClose)
            )
        }

        AnimatedVisibility(
            visible = isOpen,
            enter = slideInHorizontally(initialOffsetX = { -it }),
            exit = slideOutHorizontally(targetOffsetX = { -it })
        ) {
            AmitiaGlassSurface(
                level = GlassLevel.Navigation,
                modifier = Modifier
                    .fillMaxHeight()
                    .width(300.dp)
                    .padding(8.dp)
            ) {
                Column(
                    modifier = Modifier
                        .fillMaxSize()
                        .clip(RoundedCornerShape(AmitiaRadius.XXL))
                        .background(MaterialTheme.colorScheme.surface)
                        .padding(horizontal = 12.dp)
                        .verticalScroll(rememberScrollState())
                ) {
                    DrawerHeader(onClose = onClose)

                    DrawerRuntimeStatus()

                    amitiaDrawerSections.forEach { section ->
                        DrawerSection(
                            section = section,
                            currentDestination = currentDestination,
                            onItemClick = { route ->
                                onClose()
                                navController.navigate(route) {
                                    popUpTo(navController.graph.startDestinationId) {
                                        saveState = true
                                    }
                                    launchSingleTop = true
                                    restoreState = true
                                }
                            }
                        )
                    }

                    Spacer(modifier = Modifier.height(AmitiaSpacing.Md))

                    DrawerAccountButton(onClick = {
                        onClose()
                        onOpenProfile()
                    })

                    Spacer(modifier = Modifier.height(AmitiaSpacing.Base))
                }
            }
        }
    }
}

@Composable
private fun DrawerHeader(onClose: () -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(top = 4.dp, bottom = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(11.dp)
    ) {
        Box(
            modifier = Modifier
                .size(52.dp)
                .clip(RoundedCornerShape(18.dp))
                .background(MaterialTheme.colorScheme.primaryContainer),
            contentAlignment = Alignment.Center
        ) {
            Text(
                text = "A",
                style = MaterialTheme.typography.titleMedium,
                color = MaterialTheme.colorScheme.onPrimaryContainer,
                fontWeight = FontWeight.SemiBold
            )
            Box(
                modifier = Modifier
                    .align(Alignment.BottomEnd)
                    .size(12.dp)
                    .clip(CircleShape)
                    .background(Color(0xFF6FA176))
            )
        }
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = "Amitia",
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onSurface,
                fontWeight = FontWeight.SemiBold
            )
            Text(
                text = "默认角色",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
        Box(
            modifier = Modifier
                .size(38.dp)
                .clip(RoundedCornerShape(14.dp))
                .clickable(onClick = onClose),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = AmitiaIcons.ArrowBack,
                contentDescription = "关闭侧边栏",
                tint = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.size(AmitiaIconSize.Small)
            )
        }
    }
    Box(
        modifier = Modifier
            .fillMaxWidth()
            .height(1.dp)
            .background(MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
    )
}

@Composable
private fun DrawerRuntimeStatus() {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 13.dp, horizontal = 3.dp)
            .clip(RoundedCornerShape(14.dp))
            .background(MaterialTheme.colorScheme.primaryContainer)
            .padding(horizontal = 11.dp, vertical = 9.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(8.dp)
    ) {
        Box(
            modifier = Modifier
                .size(7.dp)
                .clip(CircleShape)
                .background(MaterialTheme.colorScheme.primary)
        )
        Text(
            text = "本地运行正常",
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.primary,
            fontSize = 10.sp
        )
    }
}

@Composable
private fun DrawerSection(
    section: AmitiaDrawerSection,
    currentDestination: androidx.navigation.NavDestination?,
    onItemClick: (String) -> Unit
) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(top = 17.dp)
    ) {
        Text(
            text = section.title,
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f),
            fontSize = 10.sp,
            modifier = Modifier.padding(horizontal = 9.dp, vertical = 7.dp)
        )
        section.items.forEach { item ->
            val isSelected = currentDestination?.hierarchy?.any { it.route == item.route || it.route?.startsWith(item.route) == true } == true
            DrawerItemRow(
                item = item,
                isSelected = isSelected,
                onClick = { onItemClick(item.route) }
            )
        }
    }
}

@Composable
private fun DrawerItemRow(
    item: AmitiaDrawerItem,
    isSelected: Boolean,
    onClick: () -> Unit
) {
    val background = if (isSelected) {
        MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.6f)
    } else {
        Color.Transparent
    }
    val textColor = if (isSelected) {
        MaterialTheme.colorScheme.onSurface
    } else {
        MaterialTheme.colorScheme.onSurfaceVariant
    }

    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 2.dp)
            .clip(RoundedCornerShape(17.dp))
            .background(background)
            .clickable(onClick = onClick)
            .padding(9.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(10.dp)
    ) {
        Box(
            modifier = Modifier
                .size(42.dp)
                .clip(RoundedCornerShape(14.dp))
                .background(MaterialTheme.colorScheme.surfaceContainer),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = item.icon,
                contentDescription = item.label,
                tint = MaterialTheme.colorScheme.primary,
                modifier = Modifier.size(AmitiaIconSize.Medium)
            )
        }
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = item.label,
                style = MaterialTheme.typography.bodyMedium,
                color = textColor,
                fontWeight = if (isSelected) FontWeight.SemiBold else FontWeight.Normal,
                fontSize = 13.sp,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
            Text(
                text = item.subtitle,
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.7f),
                fontSize = 10.sp,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
        }
    }
}

@Composable
private fun DrawerAccountButton(onClick: () -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(19.dp))
            .background(MaterialTheme.colorScheme.surface)
            .clickable(onClick = onClick)
            .padding(horizontal = 9.dp, vertical = 11.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(10.dp)
    ) {
        Box(
            modifier = Modifier
                .size(42.dp)
                .clip(RoundedCornerShape(15.dp))
                .background(
                    Brush.linearGradient(
                        listOf(
                            MaterialTheme.colorScheme.primary,
                            Color(0xFF91A697)
                        )
                    )
                ),
            contentAlignment = Alignment.Center
        ) {
            Text(
                text = "无",
                style = MaterialTheme.typography.bodyMedium,
                color = Color.White,
                fontWeight = FontWeight.SemiBold,
                fontSize = 14.sp
            )
        }
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = "无拘",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurface,
                fontWeight = FontWeight.SemiBold,
                fontSize = 13.sp
            )
            Text(
                text = "@Untrammelled",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                fontSize = 10.sp
            )
        }
        Surface(
            shape = RoundedCornerShape(50),
            color = MaterialTheme.colorScheme.primaryContainer
        ) {
            Text(
                text = "本地账号",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.primary,
                fontSize = 8.sp,
                modifier = Modifier.padding(horizontal = 7.dp, vertical = 4.dp)
            )
        }
        Icon(
            imageVector = AmitiaIcons.ChevronRight,
            contentDescription = null,
            tint = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.size(18.dp)
        )
    }
}

private object AmitiaIconSize {
    val Small = 16.dp
    val Medium = 18.dp
}
