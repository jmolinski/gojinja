package set

type FrozenSet[K comparable] map[K]struct{}

func (s FrozenSet[K]) Has(key K) bool {
	_, ok := s[key]
	return ok
}

func FrozenFromElems[K comparable](list ...K) FrozenSet[K] {
	set := make(map[K]struct{})
	for _, el := range list {
		set[el] = struct{}{}
	}
	return set
}
