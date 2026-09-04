export function estimateFrameBytes(width: number, height: number): number {
  return width * height * 4;
}

export class CacheBudgetManager {
  private budgetBytes: number;

  constructor(input: { budgetBytes: number }) {
    this.budgetBytes = input.budgetBytes;
  }

  canFit(estimatedBytes: number): boolean {
    return estimatedBytes <= this.budgetBytes;
  }

  wouldExceed(estimatedBytes: number): boolean {
    return estimatedBytes > this.budgetBytes;
  }

  setBudget(bytes: number): void {
    this.budgetBytes = bytes;
  }

  getBudget(): number {
    return this.budgetBytes;
  }

  getLowMemoryThreshold(): number {
    return Math.floor(this.budgetBytes * 0.8);
  }
}
