package service

import (
	"encoding/json"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/shopspring/decimal"
)

// harnessMediaBilling is the P1-A2 commercial explanation of one cost event.
// Amounts stay authoritative in BillingMoney; this structure only states which
// basis, how much and where the quantity came from.
type harnessMediaBilling struct {
	Basis          string
	Quantity       string
	QuantitySource string
	Components     []HarnessBillingComponent
	Expr           string
	ExprHash       string
	ExprVersion    int
}

func applyHarnessMediaBilling(correlation *harnessSettlementCorrelation, billing *harnessMediaBilling) {
	if correlation == nil || billing == nil {
		return
	}
	if billing.Basis != "" {
		correlation.BillingBasis = billing.Basis
	}
	correlation.BillingQuantity = billing.Quantity
	correlation.BillingQuantitySource = billing.QuantitySource
	if len(billing.Components) > 0 {
		if encoded, err := common.Marshal(billing.Components); err == nil {
			correlation.BillingComponents = string(encoded)
		}
	}
	correlation.BillingExpr = billing.Expr
	correlation.BillingExprHash = billing.ExprHash
	correlation.BillingExprVersion = billing.ExprVersion
}

// harnessSyncMediaBilling derives the B3 fields for a synchronous image
// request. Only the observed image count is a commercial quantity here; the
// amount itself was already produced by the billing path.
func harnessSyncMediaBilling(info *relaycommon.RelayInfo) *harnessMediaBilling {
	if info == nil || !info.MediaLedgerEnabled {
		return nil
	}
	isImageMode := info.RelayMode == relayconstant.RelayModeImagesGenerations ||
		info.RelayMode == relayconstant.RelayModeImagesEdits
	if info.MediaFacts.IsEmpty() && !isImageMode {
		return nil
	}
	billing := &harnessMediaBilling{}
	if snapshot := info.TieredBillingSnapshot; snapshot != nil {
		billing.Expr = snapshot.ExprString
		billing.ExprHash = snapshot.ExprHash
		billing.ExprVersion = snapshot.ExprVersion
	}
	if !info.MediaFacts.IsEmpty() {
		facts, err := relaycommon.ParseMediaFacts(info.MediaFacts.CanonicalJSON())
		if err != nil {
			return billing
		}
		if count, source, ok := harnessMediaFactValue(facts, relaycommon.MediaFactNameImageCount); ok {
			billing.Basis = "image"
			billing.Quantity = count
			billing.QuantitySource = source
			billing.Components = []HarnessBillingComponent{{Basis: "image", Quantity: count, Source: source}}
			return billing
		}
	}
	// An empty/aborted image response keeps the requested n as the charge, so
	// B3 explains that amount with a request-sourced quantity. This fallback is
	// limited to image endpoints; chat requests with the switch on must not grow
	// an image quantity.
	if isImageMode {
		requested := info.RequestedImageCount()
		if requested > 0 {
			quantity := decimal.NewFromInt(int64(requested)).String()
			billing.Basis = "image"
			billing.Quantity = quantity
			billing.QuantitySource = relaycommon.MediaFactSourceRequest
			billing.Components = []HarnessBillingComponent{{
				Basis:    "image",
				Quantity: quantity,
				Source:   relaycommon.MediaFactSourceRequest,
			}}
		}
	}
	return billing
}

// harnessTaskMediaBilling derives the B3 fields for an async task from the
// frozen billing context and the media facts captured at submit/completion.
// It never re-prices the task; only the explanation is derived.
func harnessTaskMediaBilling(bc *model.TaskBillingContext) *harnessMediaBilling {
	if bc == nil || !bc.MediaLedger {
		return nil
	}
	billing := &harnessMediaBilling{}
	if snapshot := bc.TieredSnapshot; snapshot != nil {
		billing.Expr = snapshot.ExprString
		billing.ExprHash = snapshot.ExprHash
		billing.ExprVersion = snapshot.ExprVersion
	}

	// Plan and fixed per-request billing: one commercial call. The expression
	// marker covers tasks whose PerCallBilling flag was not frozen because the
	// plan pool uses a tier("per_video", ...) expression.
	planExpr := bc.TieredSnapshot != nil && strings.Contains(bc.TieredSnapshot.ExprString, "per_video")
	if bc.TieredSnapshot != nil && (bc.PerCallBilling || planExpr) {
		billing.Basis = "per_call"
		billing.Quantity = "1"
		billing.QuantitySource = "request"
		billing.Components = []HarnessBillingComponent{{Basis: "per_call", Quantity: "1", Source: "request"}}
		return billing
	}

	facts, _ := relaycommon.ParseMediaFacts(bc.MediaFacts)
	expr := ""
	if bc.TieredSnapshot != nil {
		expr = bc.TieredSnapshot.ExprString
	}

	// C group: ModelPrice x request seconds, no expression. The completion may
	// observe output seconds, but the amount continues to use the request value.
	if bc.PerCallBilling && bc.TieredSnapshot == nil {
		billing.Basis = "video_second"
		if quantity, source, ok := harnessRequestSeconds(bc, facts); ok {
			setVideoBilling(billing, quantity, source)
		}
		return billing
	}

	if facts == nil {
		// An expression task may have been submitted before any fact was captured.
		// The basis and expression identity still belong on B3 even when the
		// quantity is unknown; only the quantity stays absent.
		facts = &relaycommon.MediaFacts{}
	}

	switch {
	case strings.Contains(expr, relaycommon.MediaFactNameOutputSeconds),
		strings.Contains(expr, relaycommon.MediaFactNameInputSeconds):
		// TA/A group and the input/output tier models: the billed quantity is
		// the measured output. If the upstream reported nothing, the quantity
		// stays absent rather than borrowing the request estimate, which would
		// contradict a zero-settled amount.
		billing.Basis = "video_second"
		if quantity, source, ok := harnessMediaFactValue(facts, relaycommon.MediaFactNameOutputSeconds); ok {
			billing.Quantity = quantity
			billing.QuantitySource = source
			billing.Components = []HarnessBillingComponent{{Basis: "video_second", Quantity: quantity, Source: source}}
		}
		// Kling input/output tier models also bill an input duration. The
		// component stays on the same video_second basis because the frozen
		// allowlist has no separate input basis; the quantity and source are
		// still auditable.
		if quantity, source, ok := harnessMediaFactValue(facts, relaycommon.MediaFactNameInputSeconds); ok {
			billing.Components = append(billing.Components, HarnessBillingComponent{Basis: "video_second", Quantity: quantity, Source: source})
		}
		return billing

	case strings.Contains(expr, relaycommon.MediaFactNameReferenceSeconds):
		// PAI B group: the expression bills requested + reference seconds.
		billing.Basis = "video_second"
		requested, requestedSource, hasRequested := harnessMediaFactValue(facts, relaycommon.MediaFactNameRequestedSeconds)
		if !hasRequested {
			requested, requestedSource, hasRequested = harnessRequestSeconds(bc, facts)
		}
		reference, referenceSource, hasReference := harnessMediaFactValue(facts, relaycommon.MediaFactNameReferenceSeconds)
		if !hasRequested || !hasReference {
			return billing
		}
		requestedValue, err := decimal.NewFromString(requested)
		if err != nil {
			return billing
		}
		referenceValue, err := decimal.NewFromString(reference)
		if err != nil {
			return billing
		}
		total := requestedValue.Add(referenceValue).Ceil().String()
		billing.Quantity = total
		billing.QuantitySource = harnessCombinedSource(requestedSource, referenceSource)
		billing.Components = []HarnessBillingComponent{
			{Basis: "video_second", Quantity: requested, Source: requestedSource},
			{Basis: "reference_video_second", Quantity: reference, Source: referenceSource},
		}
		return billing

	case strings.Contains(expr, "seconds"):
		// D group and any seconds-priced model: the request value is what the
		// amount was computed from.
		billing.Basis = "video_second"
		if quantity, source, ok := harnessRequestSeconds(bc, facts); ok {
			setVideoBilling(billing, quantity, source)
		}
		return billing
	}

	// No expression: use the measured output if one exists; otherwise leave the
	// quantity absent so the amount cannot imply a fabricated quantity.
	if quantity, source, ok := harnessMediaFactValue(facts, relaycommon.MediaFactNameOutputSeconds); ok {
		billing.Basis = "video_second"
		setVideoBilling(billing, quantity, source)
	}
	return billing
}

func setVideoBilling(billing *harnessMediaBilling, quantity, source string) {
	billing.Basis = "video_second"
	billing.Quantity = quantity
	billing.QuantitySource = source
	billing.Components = []HarnessBillingComponent{{Basis: "video_second", Quantity: quantity, Source: source}}
}

func harnessCombinedSource(first, second string) string {
	// The top-level source must be the weakest claim among the components:
	// an estimate anywhere makes the total estimated; otherwise request data.
	if first == relaycommon.MediaFactSourceEstimated || second == relaycommon.MediaFactSourceEstimated {
		return relaycommon.MediaFactSourceEstimated
	}
	if first == relaycommon.MediaFactSourceRequest || second == relaycommon.MediaFactSourceRequest {
		return relaycommon.MediaFactSourceRequest
	}
	return first
}

func harnessRequestSeconds(bc *model.TaskBillingContext, facts *relaycommon.MediaFacts) (string, string, bool) {
	if quantity, source, ok := harnessMediaFactValue(facts, relaycommon.MediaFactNameRequestedSeconds); ok {
		return quantity, source, true
	}
	if bc != nil {
		if seconds, ok := bc.OtherRatios["seconds"]; ok && seconds > 0 {
			return decimal.NewFromFloat(seconds).String(), relaycommon.MediaFactSourceRequest, true
		}
	}
	return "", "", false
}

func harnessMediaFactValue(facts *relaycommon.MediaFacts, names ...string) (string, string, bool) {
	if facts == nil {
		return "", "", false
	}
	for _, name := range names {
		for _, fact := range facts.Facts {
			if fact.Name == name && fact.Quantity != nil {
				return *fact.Quantity, fact.Source, true
			}
		}
	}
	return "", "", false
}

// MergeTaskCompletionMediaFacts folds the completion's usage facts into the
// task billing context before settlement. The plugin is the only producer of
// these completion facts; native adaptors expose token counts only. A key that
// the completion did not report is left untouched, so an earlier unavailable
// fact is never overwritten by a guessed zero.
func MergeTaskCompletionMediaFacts(task *model.Task, taskResult *relaycommon.TaskInfo) {
	if task == nil || taskResult == nil {
		return
	}
	bc := task.PrivateData.BillingContext
	if bc == nil || !bc.MediaLedger {
		return
	}
	facts, err := relaycommon.ParseMediaFacts(bc.MediaFacts)
	if err != nil || facts == nil {
		facts = &relaycommon.MediaFacts{}
	}
	changed := false
	// Process a fixed key order: output_seconds wins over the legacy seconds
	// key, which some providers use for the same measured duration.
	for _, key := range []string{
		relaycommon.MediaFactNameOutputSeconds,
		relaycommon.MediaFactNameInputSeconds,
		relaycommon.MediaFactNameReferenceSeconds,
		relaycommon.MediaFactNameRequestedSeconds,
		"seconds",
		"image_count",
		relaycommon.MediaFactNameImageCount,
	} {
		value, exists := taskResult.UsageFacts[key]
		if !exists {
			continue
		}
		name, dimension, unit, source, ok := harnessCompletionMediaFact(key)
		if !ok {
			continue
		}
		if key == "seconds" {
			if _, _, hasOutput := harnessMediaFactValue(facts, relaycommon.MediaFactNameOutputSeconds); hasOutput {
				continue
			}
		}
		if name == relaycommon.MediaFactNameImageCount {
			if _, _, hasImageCount := harnessMediaFactValue(facts, relaycommon.MediaFactNameImageCount); hasImageCount {
				continue
			}
		}
		quantity, ok := harnessUsageFactQuantity(value)
		if !ok {
			continue
		}
		if err := facts.SetQuantity(name, dimension, unit, source, quantity); err == nil {
			changed = true
		}
	}
	if changed {
		bc.MediaFacts = facts.CanonicalJSON()
	}
}

// harnessCompletionMediaFact maps the plugin's completion keys to the frozen
// fact names. output/input are upstream measurements; reference and requested
// seconds remain request facts; image counts are observed.
func harnessCompletionMediaFact(key string) (name, dimension, unit, source string, ok bool) {
	switch key {
	case relaycommon.MediaFactNameOutputSeconds, relaycommon.MediaFactNameInputSeconds:
		return key, relaycommon.MediaDimensionVideo, relaycommon.MediaUnitVideoSecond, relaycommon.MediaFactSourceUpstream, true
	case relaycommon.MediaFactNameReferenceSeconds:
		return key, relaycommon.MediaDimensionVideo, relaycommon.MediaUnitVideoSecond, relaycommon.MediaFactSourceRequest, true
	case "seconds":
		// Completion facts are upstream measurements. D-group providers often
		// report usage.seconds instead of output_seconds; submission-side
		// seconds are written by taskMediaFacts as requested_seconds.
		return relaycommon.MediaFactNameOutputSeconds, relaycommon.MediaDimensionVideo, relaycommon.MediaUnitVideoSecond, relaycommon.MediaFactSourceUpstream, true
	case relaycommon.MediaFactNameRequestedSeconds:
		return key, relaycommon.MediaDimensionVideo, relaycommon.MediaUnitVideoSecond, relaycommon.MediaFactSourceRequest, true
	case "image_count", relaycommon.MediaFactNameImageCount:
		return relaycommon.MediaFactNameImageCount, relaycommon.MediaDimensionImage, relaycommon.MediaUnitImage, relaycommon.MediaFactSourceObserved, true
	default:
		return "", "", "", "", false
	}
}

func harnessUsageFactQuantity(value any) (string, bool) {
	switch typed := value.(type) {
	case float64:
		return decimal.NewFromFloat(typed).String(), true
	case int:
		return decimal.NewFromInt(int64(typed)).String(), true
	case int64:
		return decimal.NewFromInt(typed).String(), true
	case json.Number:
		parsed, err := decimal.NewFromString(typed.String())
		if err != nil {
			return "", false
		}
		return parsed.String(), true
	case string:
		parsed, err := decimal.NewFromString(typed)
		if err != nil {
			return "", false
		}
		return parsed.String(), true
	default:
		return "", false
	}
}
