package hashid

type HashID struct {
	Name   string
	Short  string
	MinLen int
}

func NewType(short, name string, minLen int) *HashID {
	return &HashID{
		Name:   name,
		Short:  short,
		MinLen: minLen,
	}
}
