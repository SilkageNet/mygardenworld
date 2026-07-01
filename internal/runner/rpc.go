package runner

import (
	"encoding/json"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
)

func runnerRPC(client *babigame.Client, session *babigame.Session) *babigame.RPCClient {
	return babigame.NewRPCClient(
		client,
		session,
		babigame.WithDefaultTimeout(10*time.Second),
		babigame.WithServerErrorsAsResults(),
	)
}

func rpcResult[T any](resp babigame.RPCResponse[T], err error) (json.RawMessage, babigame.WSResponseD, error) {
	return resp.Payload, resp.Envelope, err
}
