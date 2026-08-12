package interaction

const (
	OperationStatus = "interaction.status"

	OperationClick     = "interaction.click"
	OperationLongClick = "interaction.long_click"

	OperationInputText = "interaction.input_text"
	OperationClearText = "interaction.clear_text"

	OperationScroll = "interaction.scroll"
	OperationSwipe  = "interaction.swipe"

	OperationVisualLocate = "interaction.visual_locate"
	OperationVisualClick  = "interaction.visual_click"
)

const (
	StrategyAccessibilityAction = "accessibility_action"
	StrategyNodeBounds          = "node_bounds"
	StrategyVisualOCR           = "visual_ocr"
	StrategyVisualUnderstand    = "visual_understand"
	StrategyRoot                = "root"
	StrategyADB                 = "adb"
	StrategyCoordinate          = "coordinate"
)

const (
	TargetNode        = "node"
	TargetCoordinate  = "coordinate"
	TargetVisual      = "visual"
)

const (
	AmountSmall  = "small"
	AmountMedium = "medium"
	AmountLarge  = "large"

	DirectionForward  = "forward"
	DirectionBackward = "backward"
	DirectionUp       = "up"
	DirectionDown     = "down"
	DirectionLeft     = "left"
	DirectionRight    = "right"
)

const (
	DefaultLongPressDurationMS = 600
	MinLongPressDurationMS     = 300
	MaxLongPressDurationMS     = 3000

	DefaultSwipeDurationMS = 300
	MinSwipeDurationMS     = 100
	MaxSwipeDurationMS     = 3000

	MaxInputTextRunes = 10000
)
