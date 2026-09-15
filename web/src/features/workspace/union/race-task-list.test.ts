import { describe, expect, it } from "vitest";
import { create } from "@bufbuild/protobuf";
import { FmlRaceTaskSchema } from "@/gen/mygardenworld/v1/workspace_union_pb";
import { formatRaceTaskTime, nextRaceTaskReadyAt, raceTaskAvailability, raceTaskProgressLabel, raceTaskReady, raceTaskTone, selectRaceTaskList } from "./race-task-list";

const task = (msId: number, score: number, reason = "", appearTimeMs = 0) => create(FmlRaceTaskSchema, {
  msId: BigInt(msId),
  score,
  takeSkipReason: reason,
  appearTimeMs: BigInt(appearTimeMs),
});

describe("guild race task list", () => {
  it.each([
    ["", 0, true, "ready"],
    ["", 0, false, "blocked"],
    ["冷却中", 10_000, true, "cooldown"],
    ["冷却中", 10_000, false, "cooldown"],
    ["已被接取", 10_000, true, "claimed"],
    [" 已被接取 ", 0, false, "claimed"],
    ["优先级为0", 0, true, "blocked"],
    ["12:00:10 后刷新", 10_000, true, "cooldown"],
    ["未知限制", 0, true, "blocked"],
  ])("uses an explicit presentation tone for %s", (reason, appearTimeMs, canTake, tone) => {
    expect(raceTaskTone(task(1, 30, reason, appearTimeMs), 9_000, canTake)).toBe(tone);
  });

  it("updates colors at the deadline without promoting other restrictions", () => {
    const cooling = task(1, 30, "冷却中", 10_000);
    const blocked = task(2, 30, "12:00:10 后刷新", 10_000);
    expect(raceTaskTone(cooling, 10_000, true)).toBe("ready");
    expect(raceTaskTone(cooling, 10_000, false)).toBe("blocked");
    expect(raceTaskTone(blocked, 10_000, true)).toBe("blocked");
    expect(nextRaceTaskReadyAt([blocked], 9_000)).toBe(10_000);
    expect(nextRaceTaskReadyAt([blocked], 10_000)).toBeNull();
    expect(nextRaceTaskReadyAt([task(3, 40, "已被接取", 10_000)], 9_000)).toBeNull();
  });
  it("does not label a future cooldown task as ready", () => {
    const cooling = task(1, 30, "冷却中，12:00:10 后可接", 10_000);
    expect(raceTaskReady(cooling, 9_000)).toBe(false);
    expect(raceTaskAvailability(cooling, 9_000)).toBe(`${formatRaceTaskTime(BigInt(10_000))} 后可抢`);
    expect(raceTaskAvailability(cooling, 1_000)).toBe(raceTaskAvailability(cooling, 9_000));
  });

  it("uses a fixed date and time for deadlines, including across midnight", () => {
    const ms = BigInt(new Date(2026, 8, 8, 0, 5, 10).getTime());
    expect(formatRaceTaskTime(ms)).toBe("9/8 00:05:10");
    expect(formatRaceTaskTime(BigInt(0))).toBe("");
  });

  it("only schedules the next cooldown boundary and stops when none remain", () => {
    const tasks = [task(1, 20, "冷却中", 20_000), task(2, 30, "冷却中", 10_000), task(3, 40, "优先级为0", 5_000)];
    expect(nextRaceTaskReadyAt(tasks, 0)).toBe(10_000);
    expect(nextRaceTaskReadyAt(tasks, 10_000)).toBe(20_000);
    expect(nextRaceTaskReadyAt(tasks, 20_000)).toBeNull();
    expect(nextRaceTaskReadyAt([], 0)).toBeNull();
  });

  it("promotes a cooldown task when its observed appear time arrives", () => {
    const cooling = task(1, 30, "冷却中，12:00:10 后可接", 10_000);
    expect(raceTaskReady(cooling, 10_000)).toBe(true);
    expect(raceTaskAvailability(cooling, 10_000)).toBe("现在可抢");
  });

  it("filters ready tasks and sorts score descending stably", () => {
    const tasks = [task(1, 20), task(2, 40, "优先级为0"), task(3, 40), task(4, 40)];
    expect(selectRaceTaskList(tasks, "ready", "score", 0).map(({ task }) => task.msId)).toEqual([
      BigInt(3),
      BigInt(4),
      BigInt(1),
    ]);
  });

  it("formats observed pool progress without inventing an unknown target", () => {
    expect(raceTaskProgressLabel(create(FmlRaceTaskSchema, { targetCnt: 10, finishCnt: 3 }))).toBe("进度 3/10");
    expect(raceTaskProgressLabel(create(FmlRaceTaskSchema, { finishCnt: 3 }))).toBe("已有进度 3");
    expect(raceTaskProgressLabel(create(FmlRaceTaskSchema))).toBeNull();
  });
});
