package ports

type URLValidator interface {
	Validate(raw string) error
}
