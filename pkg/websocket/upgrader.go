package ws

import (
	"fmt"
	"net/http"
)

func Upgrade(r *http.Request, w http.ResponseWriter) error {
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return fmt.Errorf("missing Sec-WebSocket-Key header")
	}
	accept := computeAcceptKey(key)

	w.Header().Set("Upgrade", "websocket")
	w.Header().Set("Connection", "Upgrade")
	w.Header().Set("Sec-WebSocket-Accept", accept)
	w.WriteHeader(http.StatusSwitchingProtocols)
	return nil
}
