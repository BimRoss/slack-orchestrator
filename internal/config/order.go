package config

import (
	"crypto/sha256"
	"encoding/binary"
	"math/rand"
	"sort"
	"strings"
)

// ResolveMultiagentOrder builds the squad roster for routing.
//
// If explicit is non-empty (MULTIAGENT_ORDER env), it is used as-is — optional operator override.
// Otherwise employee keys are taken from MULTIAGENT_BOT_USER_IDS, sorted lexicographically,
// then shuffled deterministically using secret so order is stable for a given secret + roster
// but not a fixed alex-first list in source or config.
func ResolveMultiagentOrder(explicit []string, botUserToKey map[string]string, secret string) []string {
	if len(explicit) > 0 {
		out := make([]string, len(explicit))
		copy(out, explicit)
		return out
	}
	seen := make(map[string]bool)
	var keys []string
	for _, k := range botUserToKey {
		k = strings.ToLower(strings.TrimSpace(k))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return deterministicShuffle(keys, secret)
}

func deterministicShuffle(keys []string, secret string) []string {
	if len(keys) == 0 {
		return nil
	}
	if len(keys) == 1 {
		return append([]string(nil), keys...)
	}
	out := append([]string(nil), keys...)
	h := sha256.Sum256([]byte("bimross.multiagent.roster.v1\x00" + secret + "\x00" + strings.Join(keys, ",")))
	seed := int64(binary.BigEndian.Uint64(h[:8]))
	r := rand.New(rand.NewSource(seed))
	r.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}
