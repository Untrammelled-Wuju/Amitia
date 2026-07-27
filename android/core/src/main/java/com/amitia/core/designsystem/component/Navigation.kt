package com.amitia.core.designsystem.component

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
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
import androidx.compose.foundation.layout.wrapContentHeight
import androidx.compose.foundation.layout.wrapContentWidth
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaColors
import com.amitia.core.designsystem.AmitiaElevation
import com.amitia.core.designsystem.AmitiaGlassSurface
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaNavDimensions
import com.amitia.core.designsystem.AmitiaRadius
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaBottomNavShape
import com.amitia.core.designsystem.GlassLevel
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaContentPadding
import com.amitia.core.designsystem.AmitiaPillShape
import com.amitia.core.designsystem.AmitiaTouchTarget

data class AmitiaNavItem(
    val label: String,
    val icon: ImageVector,
    val selectedIcon: ImageVector = icon,
    val route: String
)

enum class AmitiaNavBreakpoint {
    Compact, Medium, Expanded
}

object AmitiaNavBreakpoints {
    val MediumThreshold = 600.dp
    val ExpandedThreshold = 840.dp
}

@Composable
fun AmitiaBottomNavigation(
    items: List<AmitiaNavItem>,
    currentRoute: String,
    onNavigate: (String) -> Unit,
    modifier: Modifier = Modifier
) {
    Box(
        modifier = modifier
            .fillMaxWidth()
            .padding(
                horizontal = AmitiaNavDimensions.BottomNavSidePadding,
                vertical = AmitiaNavDimensions.BottomNavTopOffset
            )
    ) {
        AmitiaGlassSurface(
            level = GlassLevel.Navigation,
            modifier = Modifier.fillMaxWidth(),
            shape = AmitiaBottomNavShape
        ) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(AmitiaNavDimensions.BottomNavHeight)
                    .padding(horizontal = AmitiaSpacing.Sm),
                horizontalArrangement = Arrangement.SpaceEvenly,
                verticalAlignment = Alignment.CenterVertically
            ) {
                items.forEach { item ->
                    val selected = item.route == currentRoute
                    AmitiaBottomNavItem(
                        item = item,
                        selected = selected,
                        onClick = { onNavigate(item.route) }
                    )
                }
            }
        }
    }
}

@Composable
private fun AmitiaBottomNavItem(
    item: AmitiaNavItem,
    selected: Boolean,
    onClick: () -> Unit
) {
    val interactionSource = remember { MutableInteractionSource() }
    Column(
        modifier = Modifier
            .clip(AmitiaPillShape)
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                role = Role.Tab,
                onClick = onClick
            )
            .padding(horizontal = AmitiaSpacing.Md, vertical = AmitiaSpacing.Sm),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xxs)
    ) {
        Box(contentAlignment = Alignment.Center) {
            Icon(
                imageVector = if (selected) item.selectedIcon else item.icon,
                contentDescription = item.label,
                tint = if (selected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.size(AmitiaIconSize.Nav)
            )
        }
        Text(
            text = item.label,
            style = MaterialTheme.typography.labelSmall,
            color = if (selected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis
        )
    }
}

@Composable
fun AmitiaNavigationRail(
    items: List<AmitiaNavItem>,
    currentRoute: String,
    onNavigate: (String) -> Unit,
    modifier: Modifier = Modifier
) {
    AmitiaGlassSurface(
        level = GlassLevel.Navigation,
        modifier = modifier.fillMaxHeight(),
        shape = RoundedCornerShape(0.dp)
    ) {
        Column(
            modifier = Modifier
                .width(AmitiaNavDimensions.NavRailWidth)
                .fillMaxHeight()
                .padding(vertical = AmitiaSpacing.Base),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            items.forEach { item ->
                val selected = item.route == currentRoute
                AmitiaRailItem(
                    item = item,
                    selected = selected,
                    onClick = { onNavigate(item.route) }
                )
            }
        }
    }
}

@Composable
private fun AmitiaRailItem(
    item: AmitiaNavItem,
    selected: Boolean,
    onClick: () -> Unit
) {
    val interactionSource = remember { MutableInteractionSource() }
    Column(
        modifier = Modifier
            .clip(AmitiaPillShape)
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                role = Role.Tab,
                onClick = onClick
            )
            .padding(vertical = AmitiaSpacing.Sm),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xxs)
    ) {
        Box(
            modifier = Modifier.size(AmitiaTouchTarget.Minimum),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = if (selected) item.selectedIcon else item.icon,
                contentDescription = item.label,
                tint = if (selected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.size(AmitiaIconSize.Nav)
            )
        }
        Text(
            text = item.label,
            style = MaterialTheme.typography.labelSmall,
            color = if (selected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant,
            maxLines = 1
        )
    }
}

@Composable
fun AmitiaAdaptiveNavigation(
    items: List<AmitiaNavItem>,
    currentRoute: String,
    onNavigate: (String) -> Unit,
    modifier: Modifier = Modifier
) {
    BoxWithConstraints(modifier = modifier.fillMaxSize()) {
        val width = maxWidth
        val breakpoint = when {
            width >= AmitiaNavBreakpoints.ExpandedThreshold -> AmitiaNavBreakpoint.Expanded
            width >= AmitiaNavBreakpoints.MediumThreshold -> AmitiaNavBreakpoint.Medium
            else -> AmitiaNavBreakpoint.Compact
        }
        when (breakpoint) {
            AmitiaNavBreakpoint.Compact -> {
                Box(modifier = Modifier.fillMaxSize()) {
                    AmitiaBottomNavigation(
                        items = items,
                        currentRoute = currentRoute,
                        onNavigate = onNavigate
                    )
                }
            }
            AmitiaNavBreakpoint.Medium -> {
                Row(modifier = Modifier.fillMaxSize()) {
                    AmitiaNavigationRail(
                        items = items,
                        currentRoute = currentRoute,
                        onNavigate = onNavigate
                    )
                }
            }
            AmitiaNavBreakpoint.Expanded -> {
                Row(modifier = Modifier.fillMaxSize()) {
                    AmitiaNavigationRail(
                        items = items,
                        currentRoute = currentRoute,
                        onNavigate = onNavigate
                    )
                }
            }
        }
    }
}

@Composable
fun AmitiaTopBar(
    title: String,
    modifier: Modifier = Modifier,
    onBack: (() -> Unit)? = null,
    actions: @Composable (() -> Unit)? = null
) {
    TopAppBar(
        title = {
            Text(
                text = title,
                style = MaterialTheme.typography.titleMedium,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
        },
        navigationIcon = {
            if (onBack != null) {
                AmitiaIconButton(
                    icon = AmitiaIcons.ArrowBack,
                    contentDescription = "返回",
                    onClick = onBack
                )
            }
        },
        actions = { actions?.invoke() },
        modifier = modifier,
        colors = TopAppBarDefaults.topAppBarColors(
            containerColor = MaterialTheme.colorScheme.background,
            titleContentColor = MaterialTheme.colorScheme.onBackground,
            navigationIconContentColor = MaterialTheme.colorScheme.onBackground,
            actionIconContentColor = MaterialTheme.colorScheme.onBackground
        )
    )
}

@Composable
fun AmitiaSearchTopBar(
    query: String,
    onQueryChange: (String) -> Unit,
    modifier: Modifier = Modifier,
    placeholder: String = "搜索",
    onBack: (() -> Unit)? = null,
    onClear: (() -> Unit)? = null
) {
    Surface(
        modifier = modifier
            .fillMaxWidth()
            .height(AmitiaNavDimensions.TopBarHeight),
        color = MaterialTheme.colorScheme.background
    ) {
        Row(
            modifier = Modifier
                .fillMaxSize()
                .padding(horizontal = AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            if (onBack != null) {
                AmitiaIconButton(
                    icon = AmitiaIcons.ArrowBack,
                    contentDescription = "返回",
                    onClick = onBack
                )
            }
            Row(
                modifier = Modifier
                    .weight(1f)
                    .clip(AmitiaPillShape)
                    .background(MaterialTheme.colorScheme.surfaceVariant)
                    .padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Icon(
                    imageVector = AmitiaIcons.Search,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.size(AmitiaIconSize.Medium)
                )
                androidx.compose.material3.TextField(
                    value = query,
                    onValueChange = onQueryChange,
                    modifier = Modifier.weight(1f),
                    placeholder = {
                        Text(
                            text = placeholder,
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    },
                    singleLine = true,
                    textStyle = MaterialTheme.typography.bodyMedium.copy(color = MaterialTheme.colorScheme.onSurface),
                    colors = androidx.compose.material3.TextFieldDefaults.colors(
                        focusedContainerColor = androidx.compose.ui.graphics.Color.Transparent,
                        unfocusedContainerColor = androidx.compose.ui.graphics.Color.Transparent,
                        disabledContainerColor = androidx.compose.ui.graphics.Color.Transparent,
                        focusedIndicatorColor = androidx.compose.ui.graphics.Color.Transparent,
                        unfocusedIndicatorColor = androidx.compose.ui.graphics.Color.Transparent,
                        disabledIndicatorColor = androidx.compose.ui.graphics.Color.Transparent
                    )
                )
                if (query.isNotEmpty() && onClear != null) {
                    AmitiaIconButton(
                        icon = AmitiaIcons.Close,
                        contentDescription = "清除",
                        onClick = onClear
                    )
                }
            }
        }
    }
}

@Composable
fun AmitiaSegmentedTabs(
    tabs: List<String>,
    selectedIndex: Int,
    onSelected: (Int) -> Unit,
    modifier: Modifier = Modifier
) {
    Surface(
        modifier = modifier
            .clip(AmitiaPillShape)
            .background(MaterialTheme.colorScheme.surfaceVariant),
        color = androidx.compose.ui.graphics.Color.Transparent
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Xs),
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
        ) {
            tabs.forEachIndexed { index, tab ->
                val selected = index == selectedIndex
                val interactionSource = remember { MutableInteractionSource() }
                Box(
                    modifier = Modifier
                        .weight(1f)
                        .clip(AmitiaPillShape)
                        .background(
                            if (selected) MaterialTheme.colorScheme.surface
                            else androidx.compose.ui.graphics.Color.Transparent
                        )
                        .clickable(
                            interactionSource = interactionSource,
                            indication = null,
                            role = Role.Tab,
                            onClick = { onSelected(index) }
                        )
                        .padding(vertical = AmitiaSpacing.Sm),
                    contentAlignment = Alignment.Center
                ) {
                    Text(
                        text = tab,
                        style = MaterialTheme.typography.labelMedium,
                        color = if (selected) MaterialTheme.colorScheme.onSurface
                        else MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
        }
    }
}

@Composable
fun AmitiaBreadcrumb(
    items: List<String>,
    modifier: Modifier = Modifier,
    onNavigate: ((Int) -> Unit)? = null
) {
    Row(
        modifier = modifier
            .fillMaxWidth()
            .padding(horizontal = AmitiaContentPadding.Horizontal),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
    ) {
        items.forEachIndexed { index, item ->
            val isLast = index == items.lastIndex
            val interactionSource = remember { MutableInteractionSource() }
            Text(
                text = item,
                style = MaterialTheme.typography.labelMedium,
                color = if (isLast) MaterialTheme.colorScheme.onSurface
                else MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier
                    .clip(AmitiaPillShape)
                    .clickable(
                        interactionSource = interactionSource,
                        indication = null,
                        enabled = !isLast && onNavigate != null,
                        role = Role.Button,
                        onClick = { onNavigate?.invoke(index) }
                    )
                    .padding(horizontal = AmitiaSpacing.Xs, vertical = AmitiaSpacing.Xs)
            )
            if (!isLast) {
                Icon(
                    imageVector = AmitiaIcons.ChevronRight,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f),
                    modifier = Modifier.size(AmitiaIconSize.Small)
                )
            }
        }
    }
}

private val defaultNavItems = listOf(
    AmitiaNavItem("今日", AmitiaIcons.Today, AmitiaIcons.Today, "today"),
    AmitiaNavItem("对话", AmitiaIcons.ChatOutlined, AmitiaIcons.Chat, "chat"),
    AmitiaNavItem("角色", AmitiaIcons.PersonOutlined, AmitiaIcons.Person, "character"),
    AmitiaNavItem("记忆", AmitiaIcons.Memory, AmitiaIcons.Memory, "memory"),
    AmitiaNavItem("更多", AmitiaIcons.MoreHoriz, AmitiaIcons.MoreHoriz, "more")
)

@Preview(name = "Bottom Nav - Light", showBackground = true)
@Composable
private fun AmitiaBottomNavLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(
            modifier = Modifier.fillMaxWidth().height(120.dp),
            color = MaterialTheme.colorScheme.background
        ) {
            Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.BottomCenter) {
                AmitiaBottomNavigation(
                    items = defaultNavItems,
                    currentRoute = "today",
                    onNavigate = {}
                )
            }
        }
    }
}

@Preview(name = "Bottom Nav - Dark", showBackground = true)
@Composable
private fun AmitiaBottomNavDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(
            modifier = Modifier.fillMaxWidth().height(120.dp),
            color = MaterialTheme.colorScheme.background
        ) {
            Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.BottomCenter) {
                AmitiaBottomNavigation(
                    items = defaultNavItems,
                    currentRoute = "chat",
                    onNavigate = {}
                )
            }
        }
    }
}

@Preview(name = "TopBar - Light", showBackground = true)
@Composable
private fun AmitiaTopBarLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Column {
            AmitiaTopBar(
                title = "页面标题",
                onBack = {}
            )
            AmitiaSegmentedTabs(
                tabs = listOf("标签一", "标签二", "标签三"),
                selectedIndex = 0,
                onSelected = {}
            )
        }
    }
}
