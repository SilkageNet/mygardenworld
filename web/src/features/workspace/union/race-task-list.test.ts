import { describe, expect, it } from "vitest";
import { create } from "@bufbuild/protobuf";
import { FmlRaceTaskSchema } from "@/gen/mygardenworld/v1/workspace_union_pb";
import { raceTaskAvailability, raceTaskReady, selectRaceTaskList } from "./race-task-list";

const task = (msId: number, score: number, reason = "", appearTimeMs = 0) => create(FmlRaceTaskSchema, {
  msId: BigInt(msId),
  score,
  takeSkipReason: reason,
  appearTimeMs: BigInt(appearTimeMs),
});

describe("guild race task list", () => {
  it("does not label a future cooldown task as ready", () => {
    const cooling = task(1, 30, "冷却中，12:00:10 后可接", 10_000);
    expect(raceTaskReady(cooling, 9_000)).toBe(false);
    expect(raceTaskAvailability(cooling, 9_000)).toBe("1 秒后可抢");
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
});
