package setting

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// HarnessMediaLedgerOptionKey is the single production switch for the P1-A
// media ledger extension. It is absent by default, which means disabled.
//
// The switch is sampled once per relay (and persisted on the task billing
// context at submission), so turning it on or off affects new requests only.
// One switch controls fact capture, the B1/B3 diff and the wire fields together;
// gating only the wire would still change event counts and record versions.
const HarnessMediaLedgerOptionKey = "HarnessMediaLedgerEnabled"

func HarnessMediaLedgerEnabled() bool {
	common.OptionMapRWMutex.RLock()
	raw := ""
	if common.OptionMap != nil {
		raw = common.OptionMap[HarnessMediaLedgerOptionKey]
	}
	common.OptionMapRWMutex.RUnlock()
	enabled, err := strconv.ParseBool(strings.TrimSpace(raw))
	return err == nil && enabled
}
