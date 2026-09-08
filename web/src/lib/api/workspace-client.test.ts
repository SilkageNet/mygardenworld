import { create, toBinary } from "@bufbuild/protobuf";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MaintenanceViewSchema, WorkspaceReadySchema, WorkspaceServerFrameSchema } from "@/gen/mygardenworld/v1/workspace_pb";
import { WorkspaceClient } from "./workspace-client";

vi.mock("@/lib/api/client", () => ({
  AUTH_EXPIRED_EVENT: "test-auth-expired",
  getAccessToken: () => "unit-test-token",
  refreshAccessToken: vi.fn(),
  workspaceWebSocketUrl: () => "ws://example.invalid/api/workspace",
}));

class FakeSocket {
  static OPEN = 1;
  static latest: FakeSocket;
  readyState = 1;
  binaryType = "";
  onmessage: ((event: { data: ArrayBufferLike }) => void) | null = null;
  close = vi.fn();
  send = vi.fn();
  constructor() { FakeSocket.latest = this; }
}

afterEach(() => vi.unstubAllGlobals());

describe("maintenance workspace delivery", () => {
  it("delivers initial state and sequenced live transitions without account identifiers", async () => {
    vi.stubGlobal("WebSocket", FakeSocket);
    const ready = vi.fn();
    const changed = vi.fn();
    const client = new WorkspaceClient({ onReady: ready, onMaintenance: changed });
    client.start();
    const socket = FakeSocket.latest;
    socket.onmessage?.({ data: toBinary(WorkspaceServerFrameSchema, create(WorkspaceServerFrameSchema, {
      sequence: BigInt(1), payload: { case: "ready", value: create(WorkspaceReadySchema, {
        protocolVersion: 1, maintenance: create(MaintenanceViewSchema, { enabled: true, draining: true }),
      }) },
    })).buffer });
    for (const [sequence, enabled] of [[2, true], [3, false]] as const) {
      socket.onmessage?.({ data: toBinary(WorkspaceServerFrameSchema, create(WorkspaceServerFrameSchema, {
        sequence: BigInt(sequence), payload: { case: "maintenance", value: create(MaintenanceViewSchema, { enabled, draining: false }) },
      })).buffer });
    }
    await vi.waitFor(() => expect(changed).toHaveBeenCalledTimes(2));
    expect(ready.mock.calls[0][0].maintenance).toMatchObject({ enabled: true, draining: true });
    expect(changed.mock.calls.map(([view]) => [view.enabled, view.draining])).toEqual([[true, false], [false, false]]);
    client.stop();
  });
});
