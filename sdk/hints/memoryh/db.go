package memoryh

import (
	"fmt"
	"math"
	"slices"
	"sync"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/sdk/hints"
)

var _ hints.HintsDB = (*HintDB)(nil)

type HintDB struct {
	RelayBySerial         []string
	OrderedRelaysByPubKey map[string][]RelayEntry

	sync.Mutex
}

func NewHintDB() *HintDB {
	return &HintDB{
		RelayBySerial:         make([]string, 0, 100),
		OrderedRelaysByPubKey: make(map[string][]RelayEntry, 100),
	}
}

func (db *HintDB) Save(pubkey string, relay string, key hints.HintKey, ts nostr.Timestamp) {
	if now := nostr.Now(); ts > now {
		ts = now
	}

	relayIndex := slices.Index(db.RelayBySerial, relay)
	if relayIndex == -1 {
		relayIndex = len(db.RelayBySerial)
		db.RelayBySerial = append(db.RelayBySerial, relay)
	}

	db.Lock()
	defer db.Unlock()
	// fmt.Println(" ", relay, "index", relayIndex, "--", "adding", hints.HintKey(key).String(), ts)

	entries, _ := db.OrderedRelaysByPubKey[pubkey]

	entryIndex := slices.IndexFunc(entries, func(re RelayEntry) bool { return re.Relay == relayIndex })
	if entryIndex == -1 {
		// we don't have an entry for this relay, so add one
		entryIndex = len(entries)

		entry := RelayEntry{
			Relay: relayIndex,
		}
		entry.Timestamps[key] = ts

		entries = append(entries, entry)
	} else {
		// just update this entry
		if entries[entryIndex].Timestamps[key] < ts {
			entries[entryIndex].Timestamps[key] = ts
		} else {
			// no need to update anything
			return
		}
	}

	db.OrderedRelaysByPubKey[pubkey] = entries
}

func (db *HintDB) TopN(pubkey string, n int) []string {
	db.Lock()
	defer db.Unlock()

	urls := make([]string, 0, n)
	if entries, ok := db.OrderedRelaysByPubKey[pubkey]; ok {
		// sort everything from scratch
		slices.SortFunc(entries, func(a, b RelayEntry) int {
			return int(b.Sum() - a.Sum())
		})

		for i, re := range entries {
			urls = append(urls, db.RelayBySerial[re.Relay])
			if i+1 == n {
				break
			}
		}
	}
	return urls
}

func (db *HintDB) PrintScores() {
	db.Lock()
	defer db.Unlock()

	fmt.Println("= print scores")
	for pubkey, entries := range db.OrderedRelaysByPubKey {
		fmt.Println("== relay scores for", pubkey)
		for i, re := range entries {
			fmt.Printf("  %3d :: %30s (%3d) ::> %12d\n", i, db.RelayBySerial[re.Relay], re.Relay, re.Sum())
			// for i, ts := range re.Timestamps {
			// 	fmt.Printf("                             %-10d %s\n", ts, hints.HintKey(i).String())
			// }
		}
	}
}

func (db *HintDB) GetDetailedScores(pubkey string, n int) []hints.RelayScores {
	db.Lock()
	defer db.Unlock()

	result := make([]hints.RelayScores, 0, n)
	if entries, ok := db.OrderedRelaysByPubKey[pubkey]; ok {
		// sort everything from scratch
		slices.SortFunc(entries, func(a, b RelayEntry) int {
			return int(b.Sum() - a.Sum())
		})

		for i, re := range entries {
			if i >= n {
				break
			}
			result = append(result, hints.RelayScores{
				Relay:  db.RelayBySerial[re.Relay],
				Scores: re.Timestamps,
				Sum:    re.Sum(),
			})
		}
	}
	return result
}

type RelayEntry struct {
	Relay      int
	Timestamps [4]nostr.Timestamp
}

func (re RelayEntry) Sum() int64 {
	now := nostr.Now() + 24*60*60
	var sum int64
	for i, ts := range re.Timestamps {
		if ts == 0 {
			continue
		}

		value := float64(hints.HintKey(i).BasePoints()) * 10000000000 / math.Pow(float64(max(now-ts, 1)), 1.3)
		// fmt.Println("   ", i, "value:", value)
		sum += int64(value)
	}
	return sum
}
