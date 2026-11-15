package stdlib

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/libutil"
	"github.com/paularlott/scriptling/object"
)

// --- diff core (LCS-based) ---

type opTag string

const (
	opEqual   opTag = "equal"
	opInsert  opTag = "insert"
	opDelete  opTag = "delete"
	opReplace opTag = "replace"
)

type opcode struct {
	tag            opTag
	i1, i2, j1, j2 int
}

// lcsRowBatch is how many rows of the LCS table are allocated at once.
const lcsRowBatch = 256

// lcsTable builds the LCS dynamic programming table. The table takes
// (len(a)+1)*(len(b)+1) cells, so it is checked against the memory left
// before it is built, and rows are allocated as the work reaches them so a
// cancelled call stops allocating too. meter is charged one unit per cell.
func lcsTable(a, b []string, meter *libutil.WorkMeter) ([][]int, object.Object) {
	n, m := len(a), len(b)
	cells := (int64(n) + 1) * (int64(m) + 1)
	if errObj := object.CheckRoom("diff table", cells*int64(strconv.IntSize/8)); errObj != nil {
		return nil, errObj
	}
	dp := make([][]int, n+1)
	for i := 0; i <= n; i++ {
		if i%lcsRowBatch == 0 {
			// Rows are allocated a batch at a time as the work reaches
			// them (allocating every row with its work runs slower).
			for k := i; k < min(i+lcsRowBatch, n+1); k++ {
				dp[k] = make([]int, m+1)
			}
		}
		if i == 0 {
			continue
		}
		if errObj := meter.Add(m + 1); errObj != nil {
			return nil, errObj
		}
		prev, cur := dp[i-1], dp[i]
		ai := a[i-1]
		for j := 1; j <= m; j++ {
			if ai == b[j-1] {
				cur[j] = prev[j-1] + 1
			} else if prev[j] >= cur[j-1] {
				cur[j] = prev[j]
			} else {
				cur[j] = cur[j-1]
			}
		}
	}
	return dp, nil
}

// computeOpcodes returns opcodes describing how to turn a into b.
// Compatible with Python's SequenceMatcher.get_opcodes().
func computeOpcodes(a, b []string, meter *libutil.WorkMeter) ([]opcode, object.Object) {
	dp, errObj := lcsTable(a, b, meter)
	if errObj != nil {
		return nil, errObj
	}
	n, m := len(a), len(b)

	// Walk the DP table backwards to collect raw edit steps
	type step struct {
		op     opTag
		ai, bi int
	}
	var steps []step
	i, j := n, m
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && a[i-1] == b[j-1] {
			steps = append(steps, step{opEqual, i - 1, j - 1})
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			steps = append(steps, step{opInsert, i, j - 1})
			j--
		} else {
			steps = append(steps, step{opDelete, i - 1, j})
			i--
		}
	}

	// Reverse
	for l, r := 0, len(steps)-1; l < r; l, r = l+1, r-1 {
		steps[l], steps[r] = steps[r], steps[l]
	}

	// Collapse into opcodes
	var ops []opcode
	for _, s := range steps {
		switch s.op {
		case opEqual:
			if len(ops) > 0 && ops[len(ops)-1].tag == opEqual {
				ops[len(ops)-1].i2 = s.ai + 1
				ops[len(ops)-1].j2 = s.bi + 1
			} else {
				ops = append(ops, opcode{opEqual, s.ai, s.ai + 1, s.bi, s.bi + 1})
			}
		case opDelete:
			if len(ops) > 0 && ops[len(ops)-1].tag == opDelete {
				ops[len(ops)-1].i2 = s.ai + 1
			} else {
				ops = append(ops, opcode{opDelete, s.ai, s.ai + 1, s.bi, s.bi})
			}
		case opInsert:
			if len(ops) > 0 && ops[len(ops)-1].tag == opInsert {
				ops[len(ops)-1].j2 = s.bi + 1
			} else {
				ops = append(ops, opcode{opInsert, s.ai, s.ai, s.bi, s.bi + 1})
			}
		}
	}

	// Merge adjacent delete+insert into replace
	merged := make([]opcode, 0, len(ops))
	for k := 0; k < len(ops); k++ {
		if k+1 < len(ops) && ops[k].tag == opDelete && ops[k+1].tag == opInsert {
			merged = append(merged, opcode{opReplace, ops[k].i1, ops[k].i2, ops[k+1].j1, ops[k+1].j2})
			k++
		} else {
			merged = append(merged, ops[k])
		}
	}
	return merged, nil
}

// lcsLength returns the LCS length (used for ratio).
// meter is charged one unit per cell of the (virtual) table.
func lcsLength(a, b []string, meter *libutil.WorkMeter) (int, object.Object) {
	n, m := len(a), len(b)
	if n == 0 || m == 0 {
		return 0, nil
	}
	prev := make([]int, m+1)
	curr := make([]int, m+1)
	for i := 1; i <= n; i++ {
		if errObj := meter.Add(m); errObj != nil {
			return 0, errObj
		}
		for j := 1; j <= m; j++ {
			if a[i-1] == b[j-1] {
				curr[j] = prev[j-1] + 1
			} else if prev[j] > curr[j-1] {
				curr[j] = prev[j]
			} else {
				curr[j] = curr[j-1]
			}
		}
		prev, curr = curr, prev
		for k := range curr {
			curr[k] = 0
		}
	}
	return prev[m], nil
}

func splitLines(s string) []string {
	if s == "" {
		return []string{}
	}
	lines := strings.Split(s, "\n")
	result := make([]string, len(lines))
	for i, l := range lines {
		if i < len(lines)-1 {
			result[i] = l + "\n"
		} else {
			result[i] = l
		}
	}
	if len(result) > 0 && result[len(result)-1] == "" {
		result = result[:len(result)-1]
	}
	return result
}

func charSplit(s string) []string {
	runes := []rune(s)
	result := make([]string, len(runes))
	for i, r := range runes {
		result[i] = string(r)
	}
	return result
}

// --- unified_diff ---

func unifiedDiff(aLines, bLines []string, fromFile, toFile string, n int, meter *libutil.WorkMeter) (string, object.Object) {
	ops, errObj := computeOpcodes(aLines, bLines, meter)
	if errObj != nil {
		return "", errObj
	}

	hasChanges := false
	for _, op := range ops {
		if op.tag != opEqual {
			hasChanges = true
			break
		}
	}
	if !hasChanges {
		return "", nil
	}

	// Group ops into hunks
	type hunk struct{ ops []opcode }
	var hunks []hunk
	var cur []opcode

	flush := func() {
		if len(cur) > 0 {
			hunks = append(hunks, hunk{cur})
			cur = nil
		}
	}

	for idx, op := range ops {
		if op.tag != opEqual {
			cur = append(cur, op)
			continue
		}
		size := op.i2 - op.i1
		if size <= 2*n {
			cur = append(cur, op)
			continue
		}
		// Large equal block: take trailing context for current hunk, flush, take leading context for next
		if len(cur) > 0 || idx == 0 {
			end := op.i1 + n
			if end > op.i2 {
				end = op.i2
			}
			jend := op.j1 + n
			if jend > op.j2 {
				jend = op.j2
			}
			if len(cur) > 0 {
				cur = append(cur, opcode{opEqual, op.i1, end, op.j1, jend})
			}
		}
		flush()
		// Leading context for next hunk
		start := op.i2 - n
		if start < op.i1 {
			start = op.i1
		}
		jstart := op.j2 - n
		if jstart < op.j1 {
			jstart = op.j1
		}
		if start < op.i2 {
			cur = append(cur, opcode{opEqual, start, op.i2, jstart, op.j2})
		}
	}
	flush()

	var sb strings.Builder
	fmt.Fprintf(&sb, "--- %s\n", fromFile)
	fmt.Fprintf(&sb, "+++ %s\n", toFile)

	for _, h := range hunks {
		if len(h.ops) == 0 {
			continue
		}
		i1 := h.ops[0].i1
		i2 := h.ops[len(h.ops)-1].i2
		j1 := h.ops[0].j1
		j2 := h.ops[len(h.ops)-1].j2
		aLen := i2 - i1
		bLen := j2 - j1
		if aLen == 1 && bLen == 1 {
			fmt.Fprintf(&sb, "@@ -%d +%d @@\n", i1+1, j1+1)
		} else if aLen == 1 {
			fmt.Fprintf(&sb, "@@ -%d +%d,%d @@\n", i1+1, j1+1, bLen)
		} else if bLen == 1 {
			fmt.Fprintf(&sb, "@@ -%d,%d +%d @@\n", i1+1, aLen, j1+1)
		} else {
			fmt.Fprintf(&sb, "@@ -%d,%d +%d,%d @@\n", i1+1, aLen, j1+1, bLen)
		}
		for _, op := range h.ops {
			switch op.tag {
			case opEqual:
				for _, l := range aLines[op.i1:op.i2] {
					sb.WriteString(" ")
					sb.WriteString(l)
				}
			case opDelete:
				for _, l := range aLines[op.i1:op.i2] {
					sb.WriteString("-")
					sb.WriteString(l)
				}
			case opInsert:
				for _, l := range bLines[op.j1:op.j2] {
					sb.WriteString("+")
					sb.WriteString(l)
				}
			case opReplace:
				for _, l := range aLines[op.i1:op.i2] {
					sb.WriteString("-")
					sb.WriteString(l)
				}
				for _, l := range bLines[op.j1:op.j2] {
					sb.WriteString("+")
					sb.WriteString(l)
				}
			}
		}
	}
	return sb.String(), nil
}

// --- ratio ---

func sequenceRatio(a, b []string, meter *libutil.WorkMeter) (float64, object.Object) {
	total := len(a) + len(b)
	if total == 0 {
		return 1.0, nil
	}
	matches, errObj := lcsLength(a, b, meter)
	if errObj != nil {
		return 0, errObj
	}
	return 2.0 * float64(matches) / float64(total), nil
}

// --- get_close_matches ---

func getCloseMatches(word string, possibilities []string, n int, cutoff float64, meter *libutil.WorkMeter) ([]string, object.Object) {
	type scored struct {
		s     string
		score float64
	}
	wordChars := charSplit(word)
	var results []scored
	for _, p := range possibilities {
		r, errObj := sequenceRatio(wordChars, charSplit(p), meter)
		if errObj != nil {
			return nil, errObj
		}
		if r >= cutoff {
			results = append(results, scored{p, r})
		}
	}
	// Best first; ties keep the order of possibilities.
	sort.SliceStable(results, func(i, j int) bool { return results[i].score > results[j].score })
	if len(results) > n {
		results = results[:n]
	}
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.s
	}
	return out, nil
}

// --- Library ---

var DifflibLibrary = object.NewLibrary(DifflibLibraryName, map[string]*object.Builtin{
	"unified_diff": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 2); err != nil {
				return err
			}
			aStr, e := args[0].AsString()
			if e != nil {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			bStr, e := args[1].AsString()
			if e != nil {
				return errors.NewTypeError("STRING", args[1].Type().String())
			}
			fromFile := kwargs.MustGetString("fromfile", "")
			toFile := kwargs.MustGetString("tofile", "")
			n := int(kwargs.MustGetInt("n", 3))
			diff, errObj := unifiedDiff(splitLines(aStr), splitLines(bStr), fromFile, toFile, n, libutil.NewWorkMeter(ctx))
			if errObj != nil {
				return errObj
			}
			return object.NewString(diff)
		},
		HelpText: `unified_diff(a, b, fromfile="", tofile="", n=3) - Return a unified format diff string`,
	},
	"ratio": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			aStr, e := args[0].AsString()
			if e != nil {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			bStr, e := args[1].AsString()
			if e != nil {
				return errors.NewTypeError("STRING", args[1].Type().String())
			}
			r, errObj := sequenceRatio(charSplit(aStr), charSplit(bStr), libutil.NewWorkMeter(ctx))
			if errObj != nil {
				return errObj
			}
			return object.NewFloat(math.Round(r*100) / 100)
		},
		HelpText: `ratio(a, b) - Return a similarity ratio between 0.0 and 1.0`,
	},
	"opcodes": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			aStr, e := args[0].AsString()
			if e != nil {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			bStr, e := args[1].AsString()
			if e != nil {
				return errors.NewTypeError("STRING", args[1].Type().String())
			}
			ops, errObj := computeOpcodes(splitLines(aStr), splitLines(bStr), libutil.NewWorkMeter(ctx))
			if errObj != nil {
				return errObj
			}
			result := make([]object.Object, len(ops))
			for i, op := range ops {
				result[i] = &object.Tuple{Elements: []object.Object{
					object.NewString(string(op.tag)),
					object.NewInteger(int64(op.i1)),
					object.NewInteger(int64(op.i2)),
					object.NewInteger(int64(op.j1)),
					object.NewInteger(int64(op.j2)),
				}}
			}
			return &object.List{Elements: result}
		},
		HelpText: `opcodes(a, b) - Return list of (tag, i1, i2, j1, j2) tuples describing how to turn a into b`,
	},
	"get_close_matches": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 2); err != nil {
				return err
			}
			word, e := args[0].AsString()
			if e != nil {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			possElems, e2 := args[1].AsList()
			if e2 != nil {
				return errors.NewTypeError("LIST", args[1].Type().String())
			}
			n := int(kwargs.MustGetInt("n", 3))
			cutoff := kwargs.MustGetFloat("cutoff", 0.6)
			if len(args) >= 3 {
				if v, err := args[2].AsInt(); err == nil {
					n = int(v)
				}
			}
			if len(args) >= 4 {
				if v, err := args[3].AsFloat(); err == nil {
					cutoff = v
				}
			}
			poss := make([]string, 0, len(possElems))
			for _, p := range possElems {
				if s, err := p.AsString(); err == nil {
					poss = append(poss, s)
				}
			}
			matches, errObj := getCloseMatches(word, poss, n, cutoff, libutil.NewWorkMeter(ctx))
			if errObj != nil {
				return errObj
			}
			elems := make([]object.Object, len(matches))
			for i, m := range matches {
				elems[i] = object.NewString(m)
			}
			return &object.List{Elements: elems}
		},
		HelpText: `get_close_matches(word, possibilities, n=3, cutoff=0.6) - Return list of best matches from possibilities`,
	},
}, nil, "Helpers for computing deltas between sequences")
