export type RestoreGeometry = {
  offset: number;
  scrollTop: number;
  maxScrollTop: number;
  clientHeight: number;
};

export type RestoreResult = "exact" | "clamped";

export function classifyRestoreGeometry(target: number, geometry: RestoreGeometry | undefined): RestoreResult | undefined {
  if (!geometry || !Object.values(geometry).every(Number.isFinite)) return undefined;
  const correction = target - geometry.offset;
  if (Math.abs(correction) <= 1) return "exact";
  if ((correction < 0 && geometry.scrollTop <= 1)
    || (correction > 0 && geometry.maxScrollTop - geometry.scrollTop <= 1)) return "clamped";
  return undefined;
}

// The budget belongs to a restore job, including metadata/DOM waits, rather
// than to a render or RAF loop. Background time does not consume that budget.
export class TranscriptRestoreWindow {
  elapsed = 0;
  private lastTime: number;
  private visible: boolean;
  private sampleStart = 0;
  private samples = 0;
  private previous?: RestoreGeometry;
  private result?: RestoreResult;

  constructor(now: number, visible: boolean) {
    this.lastTime = now;
    this.visible = visible;
  }

  advance(now: number, visible: boolean): void {
    if (this.visible) this.elapsed += Math.max(0, now - this.lastTime);
    if (visible !== this.visible) this.resetSamples();
    this.lastTime = now;
    this.visible = visible;
  }

  get expired(): boolean { return this.elapsed >= 3000; }

  resetSamples(): void {
    this.previous = undefined;
    this.samples = 0;
    this.result = undefined;
  }

  sample(target: number, geometry: RestoreGeometry | undefined): RestoreResult | undefined {
    const result = classifyRestoreGeometry(target, geometry);
    if (!this.visible || !geometry || !result) {
      this.resetSamples();
      return undefined;
    }
    const stable = this.previous && result === this.result
      && (Object.keys(geometry) as (keyof RestoreGeometry)[])
        .every((key) => Math.abs(geometry[key] - this.previous![key]) <= 1);
    if (!stable) {
      this.samples = 0;
      this.sampleStart = this.elapsed;
    }
    this.previous = geometry;
    this.result = result;
    this.samples += 1;
    return this.samples >= 3 && this.elapsed - this.sampleStart >= 100 ? result : undefined;
  }
}
