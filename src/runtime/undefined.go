package runtime

import (
	"fmt"
	"github.com/gojinja/gojinja/src/errors"
	"github.com/gojinja/gojinja/src/utils"
	"reflect"
)

type Undefined struct {
	hint *string
	obj  any
	name *string
	exc  func(msg string) error
}

func (Undefined) Undefined() {}

type IUndefined interface {
	Undefined()
}

func NewUndefined(hint *string, obj any, name *string, exc func(msg string) error) Undefined {
	if exc == nil {
		exc = errors.TemplateError
	}
	return Undefined{hint, obj, name, exc}
}

func (u Undefined) undefinedMessage() string {
	if u.hint != nil {
		return *u.hint
	}
	if _, ok := u.obj.(utils.Missing); ok {
		if u.name != nil {
			return fmt.Sprintf("'%s' is undefined", *u.name)
		}
		return "'None' is undefined"
	}
	// Following if is a rewrite of python code below. I don't undeestand neither the message nor the logic, as undefined_name ought to be Optional[string].

	//if not isinstance(self._undefined_name, str):
	//return (
	//	f"{object_type_repr(self._undefined_obj)} has no"
	//f" element {self._undefined_name!r}"
	//)
	if u.name == nil {
		return fmt.Sprintf("'%s' has no element 'None'", objectTypeRepr(u.obj))
	}
	return fmt.Sprintf("'%s' has no attribute '%s'", objectTypeRepr(u.obj), *u.name)
}

func (u Undefined) failWithUndefinedError() error {
	return u.exc(u.undefinedMessage())
}

func objectTypeRepr(a any) string {
	return reflect.TypeOf(a).Name()
}
