package stdlib

import "github.com/paularlott/scriptling/object"

// Library names as constants for easy reference
const (
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
	CopyLibraryName        = "copy"
)

// RegisterAll registers all standard libraries
func RegisterAll(p interface{ RegisterLibrary(string, *object.Library) }) {
	p.RegisterLibrary(JSONLibraryName, JSONLibrary)
	p.RegisterLibrary(ReLibraryName, ReLibrary)
	p.RegisterLibrary(TimeLibraryName, TimeLibrary)
	p.RegisterLibrary(DatetimeLibraryName, DatetimeLibrary)
	p.RegisterLibrary(MathLibraryName, MathLibrary)
	p.RegisterLibrary(Base64LibraryName, Base64Library)
	p.RegisterLibrary(HashlibLibraryName, HashlibLibrary)
	p.RegisterLibrary(RandomLibraryName, RandomLibrary)
	p.RegisterLibrary(URLLibLibraryName, URLLibLibrary)
	p.RegisterLibrary(URLParseLibraryName, URLParseLibrary)
	p.RegisterLibrary(StringLibraryName, StringLibrary)
	p.RegisterLibrary(UUIDLibraryName, UUIDLibrary)
	p.RegisterLibrary(HTMLLibraryName, HTMLLibrary)
	p.RegisterLibrary(StatisticsLibraryName, StatisticsLibrary)
	p.RegisterLibrary(FunctoolsLibraryName, FunctoolsLibrary)
	p.RegisterLibrary(TextwrapLibraryName, TextwrapLibrary)
	p.RegisterLibrary(PlatformLibraryName, PlatformLibrary)
	p.RegisterLibrary(ItertoolsLibraryName, ItertoolsLibrary)
	p.RegisterLibrary(CollectionsLibraryName, CollectionsLibrary)
	p.RegisterLibrary(CopyLibraryName, CopyLibrary)
}
