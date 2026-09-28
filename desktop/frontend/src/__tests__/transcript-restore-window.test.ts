import assert from "node:assert/strict";
import { classifyRestoreGeometry, TranscriptRestoreWindow } from "../lib/transcriptRestoreWindow";

const boundary = { offset: 611.6, scrollTop: 6281.6, maxScrollTop: 6281.6, clientHeight: 600 };
assert.equal(classifyRestoreGeometry(732.2, boundary), "clamped");
assert.equal(classifyRestoreGeometry(611.6, boundary), "exact");
assert.equal(classifyRestoreGeometry(732.2, { ...boundary, scrollTop: 6100 }), undefined);
assert.equal(classifyRestoreGeometry(732.2, undefined), undefined);
assert.equal(classifyRestoreGeometry(732.2, { ...boundary, offset: NaN }), undefined);

for (const interval of [1000 / 30, 1000 / 60, 1000 / 144]) {
  const job = new TranscriptRestoreWindow(0, true);
  let result: ReturnType<typeof job.sample>;
  for (let time = 0; time < 100; time += interval) {
    job.advance(time, true);
    result = job.sample(732.2, boundary);
    assert.equal(result, undefined, "refresh rate must not shorten the measurement window");
  }
  job.advance(150, true);
  assert.equal(job.sample(732.2, boundary), "clamped");
}
const job = new TranscriptRestoreWindow(0, true);
job.advance(1000, false);
job.advance(20000, false);
assert.equal(job.elapsed, 1000);
job.advance(21000, true);
assert.equal(job.expired, false);
job.advance(23000, true);
assert.equal(job.expired, true, "visible stalls consume the remaining budget");

const unstable = new TranscriptRestoreWindow(0, true);
unstable.sample(732.2, boundary);
unstable.advance(60, true);
unstable.sample(732.2, boundary);
unstable.advance(120, true);
assert.equal(unstable.sample(732.2, { ...boundary, clientHeight: 620 }), undefined);
unstable.advance(180, true);
unstable.sample(732.2, { ...boundary, clientHeight: 620 });
unstable.advance(240, true);
assert.equal(unstable.sample(732.2, { ...boundary, clientHeight: 620 }), "clamped");
console.log("PASS transcript restore geometry, visibility budget, refresh rates and resize stability");
