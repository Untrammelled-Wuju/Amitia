export type TaskCenterReadChannel = "definitions" | "approvals" | "approval-mutation" | "devices" | "roles" | "catalog" | "submission";

export interface TaskCenterReadTicket {
  epoch: number;
  sequence: number;
}

export class TaskCenterReadFence {
  private epoch = 0;
  private sequence = new Map<TaskCenterReadChannel, number>();

  begin(channel: TaskCenterReadChannel): TaskCenterReadTicket {
    const sequence = (this.sequence.get(channel) || 0) + 1;
    this.sequence.set(channel, sequence);
    return { epoch: this.epoch, sequence };
  }

  current(channel: TaskCenterReadChannel, ticket: TaskCenterReadTicket): boolean {
    return ticket.epoch === this.epoch && this.sequence.get(channel) === ticket.sequence;
  }

  invalidate(): void {
    this.epoch++;
    this.sequence.clear();
  }
}
