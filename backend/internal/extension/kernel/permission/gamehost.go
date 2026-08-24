package permission

const (
	PermissionGameHostControl        = "gamehost.control"
	PermissionGameHostChannelUse     = "gamehost.channel.use"
	PermissionGameHostAPIInvoke      = "gamehost.host_api.invoke"
	PermissionGameHostArtifactDeploy = "gamehost.artifact.deploy"
)

type GameHostPermissionSet struct {
	Control        string
	ChannelUse     string
	APIInvoke      string
	ArtifactDeploy string
}

func GameHostPermissions() GameHostPermissionSet {
	return GameHostPermissionSet{
		Control:        PermissionGameHostControl,
		ChannelUse:     PermissionGameHostChannelUse,
		APIInvoke:      PermissionGameHostAPIInvoke,
		ArtifactDeploy: PermissionGameHostArtifactDeploy,
	}
}

func GameHostPermissionIDs() []string {
	return []string{
		PermissionGameHostControl,
		PermissionGameHostChannelUse,
		PermissionGameHostAPIInvoke,
		PermissionGameHostArtifactDeploy,
	}
}
