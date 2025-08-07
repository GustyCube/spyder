package dedup

type Interface interface {
	Seen(key string) bool
}
