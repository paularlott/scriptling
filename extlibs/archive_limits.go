package extlibs

import (
	stderrors "errors"
	"fmt"
	"io"
	"math"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// memberTooLargeError refuses an archive member over the value limit. It
// matches object.ErrValueTooLarge, so it surfaces as a MemoryError.
type memberTooLargeError struct{ msg string }

func (e *memberTooLargeError) Error() string        { return e.msg }
func (e *memberTooLargeError) Is(target error) bool { return target == object.ErrValueTooLarge }

// copyMember writes one archive member to w, held to the value limit of the
// interpreter on the calling goroutine, as reading the member with read()
// is: a member that would extract to more than the limit is refused, so a
// small archive cannot expand without bound. declared is the size the
// archive claims (negative when unknown); it is checked first, and the bytes
// actually written are checked too, so an archive that lies about sizes is
// caught. With no value limit the member is copied whole.
func copyMember(w io.Writer, r io.Reader, declared int64) error {
	max := object.MaxValueBytes()
	if max <= 0 || max == math.MaxInt64 {
		_, err := io.Copy(w, r)
		return err
	}
	tooLarge := &memberTooLargeError{fmt.Sprintf("archive entry too large: more than the %d byte limit", max)}
	if declared > max {
		return tooLarge
	}
	n, err := io.Copy(w, io.LimitReader(r, max))
	if err != nil {
		return err
	}
	if n == max {
		var probe [1]byte
		if _, err := io.ReadFull(r, probe[:]); err == nil {
			return tooLarge
		}
	}
	return nil
}

// archiveError is the script error for a failed extraction: a member over
// the value limit is a MemoryError, as from read(); anything else an
// ordinary error.
func archiveError(op string, err error) object.Object {
	if stderrors.Is(err, object.ErrValueTooLarge) {
		return object.NewMemoryError("%s: %s", op, err.Error())
	}
	return errors.NewError("%s: %s", op, err.Error())
}
