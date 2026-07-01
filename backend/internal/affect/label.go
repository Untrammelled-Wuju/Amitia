package affect

import "fmt"

const (
	labelHighThreshold = 0.55
	labelLowThreshold  = 0.35
)

type ChineseEmotionTag struct {
	Tag       string  `json:"tag"`
	Pleasure  float64 `json:"pleasure"`
	Arousal   float64 `json:"arousal"`
	Dominance float64 `json:"dominance"`
}

func ChineseEmotionLabel(pleasure, arousal, dominance float64) ChineseEmotionTag {
	p, a, d := pleasure, arousal, dominance
	tag := classifyPAD(p, a, d)
	return ChineseEmotionTag{Tag: tag, Pleasure: round4(p), Arousal: round4(a), Dominance: round4(d)}
}

func classifyPAD(pleasure, arousal, dominance float64) string {
	highP := pleasure >= labelHighThreshold
	lowP := pleasure <= labelLowThreshold
	highA := arousal >= labelHighThreshold
	lowA := arousal <= labelLowThreshold
	highD := dominance >= labelHighThreshold
	lowD := dominance <= labelLowThreshold

	if highP && highA && highD {
		return "兴奋"
	}
	if highP && highA && !highD && !lowD {
		return "愉快"
	}
	if highP && highA && lowD {
		return "激动"
	}
	if highP && !highA && !lowA && highD {
		return "满足"
	}
	if highP && !highA && !lowA && !highD && !lowD {
		return "放松"
	}
	if highP && lowA && highD {
		return "惬意"
	}
	if highP && lowA && !highD && !lowD {
		return "平静"
	}
	if highP && lowA && lowD {
		return "温和"
	}
	if !highP && !lowP && highA && highD {
		return "警觉"
	}
	if !highP && !lowP && highA && !highD && !lowD {
		return "紧张"
	}
	if !highP && !lowP && highA && lowD {
		return "焦虑"
	}
	if !highP && !lowP && !highA && !lowA && highD {
		return "自信"
	}
	if !highP && !lowP && !highA && !lowA && !highD && !lowD {
		return "平静"
	}
	if !highP && !lowP && !highA && !lowA && lowD {
		return "低落"
	}
	if !highP && !lowP && lowA && highD {
		return "沉稳"
	}
	if !highP && !lowP && lowA && !highD && !lowD {
		return "平淡"
	}
	if !highP && !lowP && lowA && lowD {
		return "低落"
	}
	if lowP && highA && highD {
		return "紧张"
	}
	if lowP && highA && !highD && !lowD {
		return "焦虑"
	}
	if lowP && highA && lowD {
		return "恐惧"
	}
	if lowP && lowA && highD {
		return "低落"
	}
	if lowP && lowA && !highD && !lowD {
		return "消沉"
	}
	if lowP && lowA && lowD {
		return "悲伤"
	}
	return fmt.Sprintf("低愉悦_%.2f_%.2f_%.2f", pleasure, arousal, dominance)
}