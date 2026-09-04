package com.amitia.amitia_app.nativeprovider.uitree

data class UITreeSnapshotRequest(
    val includeInvisible: Boolean = false,
    val maxDepth: Int = -1,
    val packageFilter: String? = null,
)

data class UITreeNode(
    val nodeId: String,
    val className: String? = null,
    val packageName: String? = null,
    val text: String? = null,
    val contentDescription: String? = null,
    val boundsInScreen: List<Int> = emptyList(),
    val clickable: Boolean = false,
    val scrollable: Boolean = false,
    val enabled: Boolean = true,
    val focused: Boolean = false,
    val children: List<String> = emptyList(),
)

data class UITreeSnapshotResult(
    val nodes: List<UITreeNode> = emptyList(),
    val windowCount: Int = 0,
    val generation: Long = 0L,
    val accessibilityConnected: Boolean = false,
)
