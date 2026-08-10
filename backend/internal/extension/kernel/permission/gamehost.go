package permission

const (
	PermissionGameHostControl    = "gamehost.control"
	PermissionGameHostChannelUse = "gamehost.channel.use"
	PermissionGameHostAPIInvoke  = "gamehost.host_api.invoke"
)

type GameHostPermissionSet struct {
	Control    string
	ChannelUse string
	APIInvoke  string
}

func GameHostPermissions() GameHostPermissionSet {
	return GameHostPermissionSet{
		Control:    PermissionGameHostControl,
		ChannelUse: PermissionGameHostChannelUse,
		APIInvoke:  PermissionGameHostAPIInvoke,
	}
}
