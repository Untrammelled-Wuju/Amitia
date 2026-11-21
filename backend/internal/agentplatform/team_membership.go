package agentplatform

import (
	"errors"
	"strings"
)

const (
	TeamMemberTypeAgent  = "agent"
	TeamMemberTypeUser   = "user"
	TeamMemberTypeDevice = "device"

	MemberOwnerCenter = "center"
	MemberOwnerDevice = "device"

	InvocationModeRole       = "role"
	InvocationModeCapability = "capability"
)

type TeamMemberInput struct {
	MemberType     string   `json:"memberType"`
	MemberID       string   `json:"memberId"`
	OwnerType      string   `json:"ownerType"`
	OwnerID        string   `json:"ownerId"`
	InvocationMode string   `json:"invocationMode"`
	RoleRevision   int64    `json:"roleRevision"`
	Role           string   `json:"role"`
	ResponderMode  string   `json:"responderMode"`
	Capabilities   []string `json:"capabilities"`
}

func normalizeTeamMemberInput(input TeamMemberInput) (TeamMemberInput, error) {
	input.MemberType = strings.ToLower(strings.TrimSpace(input.MemberType))
	input.MemberID = strings.TrimSpace(input.MemberID)
	input.OwnerType = strings.ToLower(strings.TrimSpace(input.OwnerType))
	input.OwnerID = strings.TrimSpace(input.OwnerID)
	input.InvocationMode = strings.ToLower(strings.TrimSpace(input.InvocationMode))
	input.Role = strings.TrimSpace(input.Role)
	input.ResponderMode = strings.TrimSpace(input.ResponderMode)

	if input.MemberType == "" {
		input.MemberType = TeamMemberTypeAgent
	}
	switch input.MemberType {
	case TeamMemberTypeAgent, TeamMemberTypeUser, TeamMemberTypeDevice:
	default:
		return TeamMemberInput{}, errors.New("团队成员类型必须是 agent、user 或 device")
	}
	if input.InvocationMode == "" {
		input.InvocationMode = InvocationModeRole
	}
	if input.OwnerType == "" {
		if input.MemberType == TeamMemberTypeDevice {
			input.OwnerType = MemberOwnerDevice
		} else {
			input.OwnerType = MemberOwnerCenter
		}
	}
	if input.OwnerType != MemberOwnerCenter && input.OwnerType != MemberOwnerDevice {
		return TeamMemberInput{}, errors.New("团队成员归属必须是 center 或 device")
	}
	if input.MemberType == TeamMemberTypeDevice {
		input.InvocationMode = InvocationModeCapability
		if input.OwnerType != MemberOwnerDevice || input.OwnerID == "" {
			return TeamMemberInput{}, errors.New("设备能力成员必须指定所属设备")
		}
		if input.MemberID == "" {
			input.MemberID = input.OwnerID
		}
	}
	switch input.InvocationMode {
	case InvocationModeRole, InvocationModeCapability:
	default:
		return TeamMemberInput{}, errors.New("团队成员调用模式必须是 role 或 capability")
	}
	if input.InvocationMode == InvocationModeCapability && input.MemberType != TeamMemberTypeDevice {
		return TeamMemberInput{}, errors.New("能力调用成员必须是设备成员")
	}
	if input.MemberType != TeamMemberTypeDevice && input.MemberID == "" {
		return TeamMemberInput{}, errors.New("团队成员 ID 不能为空")
	}
	if input.OwnerType == MemberOwnerDevice {
		if input.OwnerID == "" {
			return TeamMemberInput{}, errors.New("设备归属成员必须指定设备 ID")
		}
		if input.MemberType != TeamMemberTypeDevice && input.OwnerID == input.MemberID {
			return TeamMemberInput{}, errors.New("设备角色成员必须指定角色 ID")
		}
	}
	if input.OwnerType == MemberOwnerCenter && input.RoleRevision != 0 {
		return TeamMemberInput{}, errors.New("统筹中心角色成员不记录角色版本")
	}
	if input.RoleRevision < 0 {
		return TeamMemberInput{}, errors.New("角色版本不能为负数")
	}
	if input.MemberType == TeamMemberTypeUser && input.OwnerType != MemberOwnerCenter {
		return TeamMemberInput{}, errors.New("用户成员必须归属统筹中心")
	}
	if input.Role == "" {
		input.Role = "member"
	}
	if input.ResponderMode == "" {
		input.ResponderMode = "mention_or_lead"
	}
	return input, nil
}

func (input TeamMemberInput) isCenterAgent() bool {
	return input.MemberType == TeamMemberTypeAgent && input.OwnerType == MemberOwnerCenter
}

func (input TeamMemberInput) requiresDevice() bool {
	return input.OwnerType == MemberOwnerDevice
}

func teamMemberSpecKey(input TeamMemberInput) string {
	return strings.Join([]string{input.MemberType, input.OwnerType, input.OwnerID, input.MemberID, input.InvocationMode}, "|")
}
