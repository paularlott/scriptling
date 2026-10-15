package evaluator

import (
	"strings"
	"unicode"
	"unicode/utf8"
	"unsafe"

	"github.com/paularlott/scriptling/object"
)

// Size checks for operations whose result size depends on script input.
//
// object.CheckAlloc applies the interpreter's value limit and memory limit;
// these helpers add the overflow-safe size arithmetic around it and report
// new backing arrays to the memory accounting.

// absoluteMaxBytes bounds sizes even when no value limit is set, keeping all
// size arithmetic far from integer overflow (64 TiB).
const absoluteMaxBytes int64 = 1 << 46

// slotBytes is the size of one container element slot.
const slotBytes = int64(unsafe.Sizeof(object.Object(nil)))

// checkSize checks a value of n bytes. what names it in the error.
func checkSize(what string, n int64) object.Object {
	if n > absoluteMaxBytes {
		return object.NewMemoryError("%s too large: %d bytes", what, n)
	}
	return object.CheckAllocFor(what, n)
}

// checkWidth checks a padding or format width of n bytes: a width is a
// request for a single allocation, so like a repetition it is capped at
// maxRepeatBytes whatever the limits, and then checked like any value.
func checkWidth(what string, n int64) object.Object {
	if n > maxRepeatBytes {
		return object.NewMemoryError("%s too large: a width of %d is more than the %d byte maximum", what, n, int64(maxRepeatBytes))
	}
	return checkSize(what, n)
}

// checkConcat checks a concatenation producing n bytes. The fast path is a
// compare and an atomic load, so it is cheap enough for every + of strings.
func checkConcat(n int) object.Object {
	return object.CheckAllocFor("concatenation result", int64(n))
}

// checkElementCount checks a container of n elements.
func checkElementCount(n int) object.Object {
	return object.CheckElements(int64(n))
}

// Repetition result quotas: a repetition is how a script asks for a huge
// allocation in one expression, so its result is bounded even with no value
// limit set, as it always has been.
const (
	maxRepeatBytes    = 1 << 30 // 1 GiB per string/bytes repetition result
	maxRepeatElements = 1 << 27 // ~134M elements (~1 GiB of object pointers)
)

// checkRepetition validates a repetition of multiplier copies of unitLen
// units of unitBytes each (1 for strings and bytes, slotBytes for lists and
// tuples). Returns an error object to hand back from the fast paths, or nil
// when the repetition may proceed (including the trivially empty cases).
func checkRepetition(multiplier, unitLen, unitBytes int64) object.Object {
	if multiplier <= 0 || unitLen == 0 {
		return nil
	}
	quota := int64(maxRepeatBytes)
	if unitBytes != 1 {
		quota = maxRepeatElements
	}
	if multiplier > quota/unitLen {
		return object.NewMemoryError("repetition result too large (over %d units)", quota)
	}
	return checkSize("repetition result", multiplier*unitLen*unitBytes)
}

// appendElements appends extra to elems after checking the result size, and
// reports a new backing array to the memory accounting.
func appendElements(elems []object.Object, extra ...object.Object) ([]object.Object, object.Object) {
	n := len(elems) + len(extra)
	if n > cap(elems) {
		if errObj := checkElementCount(n); errObj != nil {
			return elems, errObj
		}
		elems = append(elems, extra...)
		object.TrackSlice(elems)
		return elems, nil
	}
	return append(elems, extra...), nil
}

// appendElement appends v to *elems. Appends that fit the capacity take the
// inlined fast path; growth goes through the size check and accounting.
func appendElement(elems *[]object.Object, v object.Object) object.Object {
	if len(*elems) < cap(*elems) {
		*elems = append(*elems, v)
		return nil
	}
	return growElement(elems, v)
}

// growElement is appendElement's slow path, for an append that reallocates.
func growElement(elems *[]object.Object, v object.Object) object.Object {
	if errObj := checkElementCount(len(*elems) + 1); errObj != nil {
		return errObj
	}
	*elems = append(*elems, v)
	object.TrackSlice(*elems)
	return nil
}

// checkReplaceSize checks the result of replacing up to count (all when
// negative) occurrences of old with new in s.
func checkReplaceSize(s, old, new string, count int) object.Object {
	if len(new) <= len(old) {
		return nil // the result cannot be longer than s
	}
	grow := int64(len(new) - len(old))
	// Most replacements are small: bound the result by the most matches s
	// could hold and skip counting when even that is within the limits.
	maxMatches := int64(len(s)) + 1
	if old != "" {
		maxMatches = int64(len(s) / len(old))
	}
	if count >= 0 {
		maxMatches = min(maxMatches, int64(count))
	}
	if maxMatches <= absoluteMaxBytes/grow && object.CheckAlloc(int64(len(s))+maxMatches*grow) == nil {
		return nil
	}
	var n int
	if old == "" {
		n = utf8.RuneCountInString(s) + 1
	} else {
		n = strings.Count(s, old)
	}
	if count >= 0 && count < n {
		n = count
	}
	if n == 0 {
		return nil
	}
	if int64(n) > absoluteMaxBytes/grow {
		return object.NewMemoryError("replacement result too large")
	}
	return checkSize("replacement result", int64(len(s))+int64(n)*grow)
}

// checkResult applies the value limit to what a builtin, library function or
// string/bytes method returned (see object.CheckValue).
func checkResult(res object.Object) object.Object {
	if errObj := object.CheckValue(res); errObj != nil {
		return errObj
	}
	return res
}

// checkSplitCount checks the list a split of s will build before splitting:
// sep "" means splitting on runs of whitespace, and maxsplit (negative for
// none) caps the splits. A quick bound is tried first; only a split that
// could be too large counts its separators.
func checkSplitCount(s, sep string, maxsplit int64) object.Object {
	// At most one part per separator plus one; whitespace fields need a
	// separator between them, so at most one per two bytes plus one.
	div := int64(len(sep))
	if sep == "" {
		div = 2
	}
	bound := int64(len(s))/div + 1
	if maxsplit >= 0 {
		bound = min(bound, maxsplit+1)
	}
	// Within both limits even at the most parts s could hold: no count needed.
	if object.CheckElements(bound) == nil && checkSplitPeak(len(s), bound) == nil {
		return nil
	}
	var n int64
	if sep == "" {
		n = int64(countFields(s))
	} else {
		n = int64(strings.Count(s, sep)) + 1
	}
	if maxsplit >= 0 {
		n = min(n, maxsplit+1)
	}
	if errObj := checkElementCount(int(n)); errObj != nil {
		return errObj
	}
	return checkSplitPeak(len(s), n)
}

// splitPeakPerPart is what a split holds per part while it runs: the native
// substring header, the result slot and the String object.
const splitPeakPerPart = 16 + 16 + 48

// checkSplitPeak checks the memory a split of dataLen bytes into n parts
// holds at its peak: the native parts and the result list both exist, and
// with a memory limit set each part is copied out of its source (see
// object.NewSubstring), so the data counts once more.
func checkSplitPeak(dataLen int, n int64) object.Object {
	return object.CheckRoom("split result", int64(dataLen)+n*splitPeakPerPart)
}

// countFields counts the whitespace-separated fields in s.
func countFields(s string) int {
	n := 0
	inField := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			inField = false
		} else if !inField {
			inField = true
			n++
		}
	}
	return n
}

// checkLineCount checks the list splitlines will build from s.
func checkLineCount(s string) object.Object {
	if s == "" {
		return nil // no lines
	}
	if object.CheckElements(int64(len(s))+1) == nil && checkSplitPeak(len(s), int64(len(s))+1) == nil {
		return nil
	}
	// One line per break (\r\n is one break), plus any text after the last.
	n := strings.Count(s, "\n") + strings.Count(s, "\r") - strings.Count(s, "\r\n")
	if last := s[len(s)-1]; last != '\n' && last != '\r' {
		n++
	}
	if errObj := checkElementCount(n); errObj != nil {
		return errObj
	}
	return checkSplitPeak(len(s), int64(n))
}

// bytesPartBytes is what a bytes split holds per part beyond the data: the
// native slice header, the result slot and the Bytes object.
const bytesPartBytes = 24 + 16 + 48

// checkBytesSplit checks a bytes split of data into up to bound parts,
// counting the exact number with exact only when the bound alone might be
// too large. At its peak the split holds the parts copied out of data (a
// Bytes value owns its copy) beside the source, plus the headers, the result
// list and the objects.
func checkBytesSplit(dataLen int, bound int64, exact func() int) object.Object {
	peak := func(n int64) int64 { return int64(dataLen) + n*bytesPartBytes }
	if object.CheckElements(bound) == nil && object.CheckRoom("split result", peak(bound)) == nil {
		return nil
	}
	n := int64(exact())
	if errObj := checkElementCount(int(n)); errObj != nil {
		return errObj
	}
	return object.CheckRoom("split result", peak(n))
}

// checkDictGrowth checks a dict about to store key hk: when the key is new
// the dict grows by one entry, held to the value limit like a list of the
// same length. The inlined fast path skips the membership test.
func checkDictGrowth(d *object.Dict, hk string) object.Object {
	n := len(d.Pairs) + 1
	if object.AllocWithin(int64(n) * objectSlotBytes) {
		return nil
	}
	if _, exists := d.Pairs[hk]; exists {
		return nil
	}
	return checkGrowthTo("dict", n, object.DictEntryWeight)
}

const objectSlotBytes = int64(unsafe.Sizeof(object.Object(nil)))

// checkGrowthTo checks a dict or set about to hold n entries: the container
// against the value limit, and the memory the growth takes, which is the
// next doubling of the map when n reaches one and a single entry otherwise,
// since what the container holds is already counted.
func checkGrowthTo(what string, n int, entryWeight int64) object.Object {
	if errObj := object.CheckSlots(int64(n)); errObj != nil {
		return errObj
	}
	grow := int64(1)
	if n&(n-1) == 0 {
		grow = int64(n)
	}
	return object.CheckRoom(what, grow*entryWeight)
}

// checkDictSize checks a dict about to be presized for n entries, in the
// units it is charged in: n element slots against the value limit, and
// n dict entries (order slot plus map bucket share) against the memory left.
func checkDictSize(n int) object.Object {
	if errObj := checkElementCount(n); errObj != nil {
		return errObj
	}
	return object.CheckRoom("dict", int64(n)*object.DictEntryWeight)
}

// checkDictMerge checks merging other into d.
func checkDictMerge(d, other *object.Dict) object.Object {
	if object.CheckElements(int64(len(d.Pairs)+len(other.Pairs))) == nil {
		return nil
	}
	n := len(d.Pairs)
	for k := range other.Pairs {
		if _, exists := d.Pairs[k]; !exists {
			n++
		}
	}
	return checkDictSize(n)
}

// checkSetGrowth checks a set about to gain the element keyed hk.
func checkSetGrowth(s *object.Set, hk string) object.Object {
	n := len(s.Elements) + 1
	if object.AllocWithin(int64(n) * objectSlotBytes) {
		return nil
	}
	if s.ContainsKeyed(hk) {
		return nil
	}
	return checkGrowthTo("set", n, object.SetEntryWeight)
}

// checkSetMerge checks adding every element of other to s.
func checkSetMerge(s, other *object.Set) object.Object {
	if object.CheckElements(int64(len(s.Elements)+len(other.Elements))) == nil {
		return nil
	}
	n := len(s.Elements)
	for k := range other.Elements {
		if !s.ContainsKeyed(k) {
			n++
		}
	}
	return checkElementCount(n)
}

// maxPresize caps how far a comprehension presizes its result from the
// length of what it iterates. A huge range would otherwise allocate its
// whole result before the first element, beyond any timeout's reach; past
// the cap the result grows as elements arrive.
const maxPresize = 1 << 16

// clampMaxsplit turns a maxsplit too large to matter (more splits than the
// string has bytes) into -1, no limit, so maxsplit+1 can never overflow.
func clampMaxsplit(maxsplit int64, length int) int64 {
	if maxsplit > int64(length) {
		return -1
	}
	return maxsplit
}
