package runner

import (
	"encoding/json"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientrpc"
)

func (r *Runner) runnerRPC(client *babigame.Client, session *babigame.Session) *clientrpc.Client {
	return clientrpc.NewClient(r.runnerRawRPC(client, session))
}

func (r *Runner) runnerRawRPC(client *babigame.Client, session *babigame.Session) *babigame.RPCClient {
	return babigame.NewRPCClient(
		client,
		session,
		babigame.WithDefaultTimeout(10*time.Second),
		babigame.WithServerErrorsAsResults(),
		babigame.WithApplyV(r.state.ApplyV),
	)
}

func rpcResult[T any](resp babigame.RPCResponse[T], err error) (json.RawMessage, babigame.WSResponseD, error) {
	return resp.Payload, resp.Envelope, err
}
