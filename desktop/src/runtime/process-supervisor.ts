import type {
  ManagedProcessDefinition,
  ManagedProcessStatus,
} from "../shared/types";

export interface ProcessSupervisor {
  getStatus(id: string): ManagedProcessStatus;
  start(definition: ManagedProcessDefinition): Promise<ManagedProcessStatus>;
  stop(id: string): Promise<ManagedProcessStatus>;
  fail(id: string, error: string, exitCode?: number): ManagedProcessStatus;
}

export class InMemoryProcessSupervisor implements ProcessSupervisor {
  private statuses = new Map<string, ManagedProcessStatus>();

  getStatus(id: string): ManagedProcessStatus {
    return this.statuses.get(id) || { id, state: "stopped" };
  }

  async start(
    definition: ManagedProcessDefinition,
  ): Promise<ManagedProcessStatus> {
    const starting: ManagedProcessStatus = {
      id: definition.id,
      state: "starting",
    };
    this.statuses.set(definition.id, starting);
    const running: ManagedProcessStatus = {
      id: definition.id,
      state: "running",
      pid: 0,
    };
    this.statuses.set(definition.id, running);
    return running;
  }

  async stop(id: string): Promise<ManagedProcessStatus> {
    const current = this.getStatus(id);
    if (current.state === "stopped") return current;
    const stopping: ManagedProcessStatus = { ...current, state: "stopping" };
    this.statuses.set(id, stopping);
    const stopped: ManagedProcessStatus = { id, state: "stopped", exitCode: 0 };
    this.statuses.set(id, stopped);
    return stopped;
  }

  fail(id: string, error: string, exitCode = 1): ManagedProcessStatus {
    const failed: ManagedProcessStatus = {
      id,
      state: "failed",
      error,
      exitCode,
    };
    this.statuses.set(id, failed);
    return failed;
  }
}
