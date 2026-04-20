package data

// Page contains a paginated repository result and its metadata.
type Page[T any] struct {
	Items    []T
	Total    int
	Page     int
	PageSize int
}
