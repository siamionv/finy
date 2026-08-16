package cerr

// kind classifies a failure. It is unexported so Error() can skip kind values
// when rendering a message, while errors.Is still matches them.
type kind string

func (k kind) Error() string { return string(k) }

// Classification vocabulary. A domain sentinel wraps one of these, so
// errors.Is(err, cerr.NotFound) answers "what class of failure is this"
// across every package.
const (
	NotFound     kind = "not found"
	Conflict     kind = "conflict"
	Invalid      kind = "invalid argument"
	Denied       kind = "permission denied"
	Unauthorized kind = "unauthenticated"
	RateLimited  kind = "rate limited"
	Unavailable  kind = "unavailable"
	Timeout      kind = "timeout"
	Internal     kind = "internal"
)
