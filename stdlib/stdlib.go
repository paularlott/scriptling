package stdlib

import "github.com/paularlott/scriptling/object"

// Library names as constants for easy reference
const (
	IOLibraryName          = "io"
	JSONLibraryName        = "json"
	ReLibraryName          = "re"
	TimeLibraryName        = "time"
	DatetimeLibraryName    = "datetime"
	MathLibraryName        = "math"
	Base64LibraryName      = "base64"
	HashlibLibraryName     = "hashlib"
	RandomLibraryName      = "random"
	URLLibLibraryName      = "urllib"
	URLParseLibraryName    = "urllib.parse"
	StringLibraryName      = "string"
	UUIDLibraryName        = "uuid"
	HTMLLibraryName        = "html"
	StatisticsLibraryName  = "statistics"
	FunctoolsLibraryName   = "functools"
	TextwrapLibraryName    = "textwrap"
	PlatformLibraryName    = "platform"
	ItertoolsLibraryName   = "itertools"
	CollectionsLibraryName = "collections"
)

// RegisterAll registers all standard libraries
func RegisterAll(p interface{ RegisterLibrary(*object.Library) }) {
	p.RegisterLibrary(JSONLibrary)
	p.RegisterLibrary(ReLibrary)
	p.RegisterLibrary(TimeLibrary)
	p.RegisterLibrary(DatetimeLibrary)
	p.RegisterLibrary(MathLibrary)
	p.RegisterLibrary(Base64Library)
	p.RegisterLibrary(HashlibLibrary)
	p.RegisterLibrary(RandomLibrary)
	p.RegisterLibrary(URLLibLibrary)
	p.RegisterLibrary(URLParseLibrary)
	p.RegisterLibrary(StringLibrary)
	p.RegisterLibrary(UUIDLibrary)
	p.RegisterLibrary(HTMLLibrary)
	p.RegisterLibrary(StatisticsLibrary)
	p.RegisterLibrary(FunctoolsLibrary)
	p.RegisterLibrary(TextwrapLibrary)
	p.RegisterLibrary(PlatformLibrary)
	p.RegisterLibrary(ItertoolsLibrary)
	p.RegisterLibrary(CollectionsLibrary)
	p.RegisterLibrary(IOLibrary)
}
