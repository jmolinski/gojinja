package tests

type Test func(...any) bool

var Default = map[string]Test{}
