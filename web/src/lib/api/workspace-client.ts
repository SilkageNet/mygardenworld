import { create, fromBinary, toBinary } from "@bufbuild/protobuf";
import { AlipayLoginStatus } from "@/gen/mygardenworld/v1/account_pb";
import {
  AccountRedeemAttemptFilter,
  WorkspaceClientFrameSchema,
  WorkspaceServerFrameSchema,
  LoadAccountRedeemAttemptsSchema,
  LoadWorkspaceLogsSchema,
  OpenWorkspaceSchema,
  ResyncWorkspaceSchema,
  SelectWorkspaceAccountSchema,
  WatchAlipayLoginSchema,
  type AccountStatusBatch,
  type AccountRedeemAttemptPage,
  type AlipayLoginProgress,
  type WorkspaceClientFrame,
  type WorkspaceError,
  type WorkspaceLogPage,
  type WorkspacePatch,
  type WorkspaceReady,
  type WorkspaceSnapshot,
} from "@/gen/mygardenworld/v1/workspace_pb";
import {
  AUTH_EXPIRED_EVENT,
  getAccessToken,
  refreshAccessToken,
  workspaceWebSocketUrl,
} from "@/lib/api/client";

export const WORKSPACE_PROTOCOL_VERSION = 1;

const RECONNECT_INITIAL_MS = 1_000;
const RECONNECT_MAX_MS = 15_000;

export type WorkspaceConnectionState = "connecting" | "open" | "closed";

export type WorkspaceClientHandlers = {
  onConnectionState?: (state: WorkspaceConnectionState) => void;
  onReady?: (ready: WorkspaceReady) => void;
  onStatuses?: (batch: AccountStatusBatch) => void;
  onSnapshot?: (snapshot: WorkspaceSnapshot) => void;
  onPatch?: (patch: WorkspacePatch) => void;
  onLogs?: (page: WorkspaceLogPage) => void;
  onRedeemAttempts?: (page: AccountRedeemAttemptPage) => void;
  onAlipayLogin?: (progress: AlipayLoginProgress) => void;
  onError?: (error: WorkspaceError) => void;
};

export class WorkspaceClient {
  private socket: WebSocket | null = null;
  private stopped = true;
  private reconnectTimer: number | null = null;
  private reconnectDelayMs = RECONNECT_INITIAL_MS;
  private selectedAccountId = "";
  private alipayLoginId = "";
  private requestId = BigInt(0);
  private lastSequence = BigInt(0);
  private logCursors = new Map<string, bigint>();
  private messageChain: Promise<void> = Promise.resolve();

  constructor(private readonly handlers: WorkspaceClientHandlers) {}

  start(selectedAccountId = "") {
    this.selectedAccountId = selectedAccountId;
    if (!this.stopped) return;
    this.stopped = false;
    this.connect();
  }

  stop() {
    this.stopped = true;
    if (this.reconnectTimer !== null) {
      window.clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    const socket = this.socket;
    this.socket = null;
    if (socket && socket.readyState < WebSocket.CLOSING) {
      socket.close(1000, "workspace closed");
    }
    this.handlers.onConnectionState?.("closed");
  }

  selectAccount(accountId: string) {
    this.selectedAccountId = accountId;
    if (!accountId) return;
    this.send({
      case: "selectAccount",
      value: create(SelectWorkspaceAccountSchema, {
        accountId: BigInt(accountId),
        afterLogId: this.logCursors.get(accountId) ?? BigInt(0),
      }),
    });
  }

  resync() {
    if (!this.selectedAccountId) return;
    this.send({
      case: "resync",
      value: create(ResyncWorkspaceSchema, {
        afterLogId: this.logCursors.get(this.selectedAccountId) ?? BigInt(0),
      }),
    });
  }

  loadLogs(accountId: string, beforeId = BigInt(0), limit = 200) {
    if (!accountId) return false;
    return this.send({
      case: "loadLogs",
      value: create(LoadWorkspaceLogsSchema, { accountId: BigInt(accountId), beforeId, limit }),
    });
  }

  loadRedeemAttempts(
    accountId: string,
    beforeId = BigInt(0),
    limit = 20,
    filter = AccountRedeemAttemptFilter.ALL,
  ) {
    if (!accountId) return false;
    return this.send({
      case: "loadRedeemAttempts",
      value: create(LoadAccountRedeemAttemptsSchema, {
        accountId: BigInt(accountId),
        beforeId,
        limit,
        filter,
      }),
    });
  }

  watchAlipayLogin(loginId: string) {
    if (!loginId) return;
    this.alipayLoginId = loginId;
    this.send({ case: "watchAlipayLogin", value: create(WatchAlipayLoginSchema, { loginId }) });
  }

  private connect() {
    if (this.stopped || this.socket) return;
    const token = getAccessToken();
    if (!token) {
      this.scheduleReconnect(true);
      return;
    }
    this.handlers.onConnectionState?.("connecting");
    const socket = new WebSocket(workspaceWebSocketUrl());
    socket.binaryType = "arraybuffer";
    this.socket = socket;

    socket.onopen = () => {
      if (this.socket !== socket || this.stopped) return;
      this.lastSequence = BigInt(0);
      this.reconnectDelayMs = RECONNECT_INITIAL_MS;
      this.handlers.onConnectionState?.("open");
      this.send({
        case: "open",
        value: create(OpenWorkspaceSchema, {
          protocolVersion: WORKSPACE_PROTOCOL_VERSION,
          accessToken: token,
          selectedAccountId: this.selectedAccountId ? BigInt(this.selectedAccountId) : BigInt(0),
          afterLogId: this.selectedAccountId
            ? (this.logCursors.get(this.selectedAccountId) ?? BigInt(0))
            : BigInt(0),
        }),
      });
      if (this.alipayLoginId) {
        this.send({
          case: "watchAlipayLogin",
          value: create(WatchAlipayLoginSchema, { loginId: this.alipayLoginId }),
        });
      }
    };
    socket.onmessage = (event) => {
      if (this.socket !== socket || this.stopped) return;
      // Blob conversion is asynchronous. Chaining prevents a smaller later
      // frame from overtaking an earlier one before sequence validation.
      this.messageChain = this.messageChain
        .then(() => this.handleMessage(socket, event.data))
        .catch(() => {
          if (this.socket !== socket || this.stopped) return;
          this.reportInvalidFrame("无法读取服务端状态消息");
        });
    };
    socket.onerror = () => {
      // onclose owns reconnect scheduling so every failure has one retry path.
    };
    socket.onclose = (event) => {
      if (this.socket === socket) this.socket = null;
      if (this.stopped) return;
      this.handlers.onConnectionState?.("closed");
      this.scheduleReconnect(event.code === 4401);
    };
  }

  private async handleMessage(source: WebSocket, raw: unknown) {
    const bytes = await binaryPayload(raw);
    if (this.socket !== source || this.stopped) return;
    if (!bytes) {
      this.reportInvalidFrame("服务端返回了非二进制消息");
      return;
    }
    let frame;
    try {
      frame = fromBinary(WorkspaceServerFrameSchema, bytes);
    } catch {
      this.reportInvalidFrame("无法解析服务端状态消息");
      return;
    }

    if (this.lastSequence > BigInt(0) && frame.sequence !== this.lastSequence + BigInt(1)) {
      this.lastSequence = frame.sequence;
      this.resync();
      return;
    }
    this.lastSequence = frame.sequence;

    const payload = frame.payload;
    switch (payload.case) {
      case "ready":
        if (payload.value.protocolVersion !== WORKSPACE_PROTOCOL_VERSION) {
          this.handlers.onError?.({
            $typeName: "mygardenworld.v1.WorkspaceError",
            code: "protocol_version_mismatch",
            message: "前后端工作区协议版本不一致",
            retryable: false,
          });
          this.stop();
          return;
        }
        this.handlers.onReady?.(payload.value);
        break;
      case "accountStatuses":
        this.handlers.onStatuses?.(payload.value);
        break;
      case "snapshot":
        this.noteLogs(payload.value.logs?.events ?? []);
        this.handlers.onSnapshot?.(payload.value);
        break;
      case "patch":
        this.handlers.onPatch?.(payload.value);
        break;
      case "logs":
        this.noteLogs(payload.value.events);
        this.handlers.onLogs?.(payload.value);
        break;
      case "redeemAttempts":
        this.handlers.onRedeemAttempts?.(payload.value);
        break;
      case "alipayLogin":
        if (
          payload.value.status === AlipayLoginStatus.COMPLETE ||
          payload.value.status === AlipayLoginStatus.EXPIRED ||
          payload.value.status === AlipayLoginStatus.FAILED
        ) {
          this.alipayLoginId = "";
        }
        this.handlers.onAlipayLogin?.(payload.value);
        break;
      case "error":
        this.handlers.onError?.(payload.value);
        break;
    }
  }

  private reportInvalidFrame(message: string) {
    this.handlers.onError?.({
      $typeName: "mygardenworld.v1.WorkspaceError",
      code: "invalid_server_frame",
      message,
      retryable: true,
    });
    this.resync();
  }

  private noteLogs(events: Array<{ accountId: bigint; id: bigint }>) {
    for (const event of events) {
      if (event.id <= BigInt(0)) continue;
      const key = event.accountId.toString();
      const current = this.logCursors.get(key) ?? BigInt(0);
      if (event.id > current) this.logCursors.set(key, event.id);
    }
  }

  private send(payload: WorkspaceClientFrame["payload"]) {
    const socket = this.socket;
    if (!socket || socket.readyState !== WebSocket.OPEN) return false;
    this.requestId += BigInt(1);
    const frame = create(WorkspaceClientFrameSchema, { requestId: this.requestId, payload });
    socket.send(toBinary(WorkspaceClientFrameSchema, frame));
    return true;
  }

  private scheduleReconnect(refreshToken: boolean) {
    if (this.stopped || this.reconnectTimer !== null) return;
    const delay = this.reconnectDelayMs;
    this.reconnectDelayMs = Math.min(this.reconnectDelayMs * 2, RECONNECT_MAX_MS);
    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = null;
      void (async () => {
        if (refreshToken) {
          const refreshed = await refreshAccessToken();
          if (!refreshed) {
            window.dispatchEvent(new Event(AUTH_EXPIRED_EVENT));
            return;
          }
        }
        this.connect();
      })();
    }, delay);
  }
}

async function binaryPayload(raw: unknown): Promise<Uint8Array | null> {
  if (raw instanceof ArrayBuffer) return new Uint8Array(raw);
  if (raw instanceof Blob) return new Uint8Array(await raw.arrayBuffer());
  return null;
}
