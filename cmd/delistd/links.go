package main

import (
	"fmt"

	"github.com/drumandbytes/delistd/internal/history"
)

// storedLink is a link a broker's reply carried, and the broker it came from.
type storedLink struct {
	BrokerID string
	URL      string
}

// storedLinksFor picks the links a command should act on: the one it was
// given, one broker's newest from the stored replies of that type, or all of
// them. The command needs at least one of direct, brokerID and pending.
func storedLinksFor(store *history.Store, profileID, replyType, direct, brokerID string, pending bool,
	urlOf func(history.BrokerResponse) string, noneForBroker string) ([]storedLink, error) {
	if direct != "" {
		return []storedLink{{BrokerID: brokerID, URL: direct}}, nil
	}
	if brokerID == "" && !pending {
		return nil, fmt.Errorf("please specify --url, --broker, or --pending")
	}
	replies, err := store.GetBrokerResponses(profileID, replyType, false, 100)
	if err != nil {
		return nil, fmt.Errorf("failed to get broker responses: %w", err)
	}
	var links []storedLink
	for _, r := range replies {
		u := urlOf(r)
		if u == "" || (brokerID != "" && r.BrokerID != brokerID) {
			continue
		}
		links = append(links, storedLink{BrokerID: r.BrokerID, URL: u})
		if brokerID != "" {
			break // the newest reply for that broker is the one to use
		}
	}
	if brokerID != "" && len(links) == 0 {
		return nil, fmt.Errorf("%s: %s", noneForBroker, brokerID)
	}
	return links, nil
}
