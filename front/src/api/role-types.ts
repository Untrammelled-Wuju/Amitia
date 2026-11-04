export interface AgentProfile {
  id: string;
  spaceId: string;
  characterId: string;
  name: string;
  description: string;
  archetype: string;
  status: string;
  ownerId: string;
  teamId: string;
  currentRevisionId: string;
  currentReleaseId: string;
  revision: number;
  createdAt: string;
  updatedAt: string;
}

export interface Team {
  id: string;
  name: string;
  description: string;
  leadAgentId: string;
  workspaceId: string;
  workspaceRootUri: string;
  sharedMemory: boolean;
  sharedWorkspace: boolean;
  status: string;
  revision: number;
}
