package config

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math/rand"
	"sort"
	"strings"
)

// DerivedShuffleSeed builds a stable seed from the bot roster map so roster shuffle and plain-message
// routing do not require MULTIAGENT_SHUFFLE_SECRET. Same roster → same seed across restarts.
func DerivedShuffleSeed(botUserToKey map[string]string) string {
	if len(botUserToKey) == 0 {
		return "bimross-no-roster-v1"
	}
	var uids []string
	for uid := range botUserToKey {
		uids = append(uids, uid)
	}
	sort.Strings(uids)
	var b strings.Builder
	for i, uid := range uids {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(uid)
		b.WriteByte('=')
		b.WriteString(strings.ToLower(strings.TrimSpace(botUserToKey[uid])))
	}
	sum := sha256.Sum256([]byte("bimross.shuffle.seed.v1\x00" + b.String()))
	return hex.EncodeToString(sum[:])
}

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
