package runtime

import (
	"fmt"
	"github.com/gojinja/gojinja/src/errors"
	"github.com/gojinja/gojinja/src/utils"
	"log"
	"reflect"
	"strconv"
)

type Undefined struct {
	hint   *string
	obj    any
	name   *string
	exc    func(msg string) error
	logger *log.Logger
}

type StrictUndefined struct {
	Undefined
}

type DebugUndefined struct {
	Undefined
}

type ChainableUndefined struct {
	Undefined
}

func (Undefined) Undefined() {}

type IUndefined interface {
	Undefined()
}

func NewUndefined(hint *string, obj any, name *string, exc func(msg string) error, logger *log.Logger) Undefined {
	if exc == nil {
		exc = errors.TemplateError
	}
	return Undefined{hint, obj, name, exc, logger}
}

func NewStrictUndefined(hint *string, obj any, name *string, exc func(msg string) error, logger *log.Logger) StrictUndefined {
	return StrictUndefined{NewUndefined(hint, obj, name, exc, logger)}
}

func NewChainableUndefined(hint *string, obj any, name *string, exc func(msg string) error, logger *log.Logger) ChainableUndefined {
	return ChainableUndefined{NewUndefined(hint, obj, name, exc, logger)}
}

func NewDebugUndefined(hint *string, obj any, name *string, exc func(msg string) error, logger *log.Logger) StrictUndefined {
	return StrictUndefined{NewUndefined(hint, obj, name, exc, logger)}
}

func (u Undefined) undefinedMessage() string {
	if u.hint != nil {
		return *u.hint
	}
	if _, ok := u.obj.(utils.Missing); ok {
		return fmt.Sprintf("'%s' is undefined", *u.name)
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

func (u Undefined) logMessage() {
	if u.logger != nil {
		u.logger.Printf("Template variable warning: %s", u.undefinedMessage())
	}
}

func (u Undefined) failWithUndefinedError() error {
	err := u.exc(u.undefinedMessage())
	if u.logger != nil {
		log.Printf("Template variable error: %v", err)
	}
	return err
}

func (u Undefined) Add(any) (any, error) {
	return nil, u.failWithUndefinedError()
}

func (u Undefined) RAdd(any) (any, error) {
	return nil, u.failWithUndefinedError()
}

func (u Undefined) Sub(any) (any, error) {
	return nil, u.failWithUndefinedError()
}

func (u Undefined) RSub(any) (any, error) {
	return nil, u.failWithUndefinedError()
}

func (u Undefined) Mul(any) (any, error) {
	return nil, u.failWithUndefinedError()
}

func (u Undefined) RMul(any) (any, error) {
	return nil, u.failWithUndefinedError()
}

func (u Undefined) Div(any) (any, error) {
	return nil, u.failWithUndefinedError()
}

func (u Undefined) RDiv(any) (any, error) {
	return nil, u.failWithUndefinedError()
}

func (u Undefined) FloorDiv(any) (any, error) {
	return nil, u.failWithUndefinedError()
}

func (u Undefined) RFloorDiv(any) (any, error) {
	return nil, u.failWithUndefinedError()
}

func (u Undefined) Mod(any) (any, error) {
	return nil, u.failWithUndefinedError()
}

func (u Undefined) RMod(any) (any, error) {
	return nil, u.failWithUndefinedError()
}

func (u Undefined) Eq(a any) (any, error) {
	return reflect.TypeOf(a).Name() == reflect.TypeOf(u).Name(), nil
}

func (u Undefined) Ne(a any) (any, error) {
	eq, err := u.Eq(a)
	return !eq.(bool), err
}

func (u Undefined) Lt(any) (any, error) {
	return nil, u.failWithUndefinedError()
}

func (u Undefined) Le(any) (any, error) {
	return nil, u.failWithUndefinedError()
}

func (u Undefined) Gt(any) (any, error) {
	return nil, u.failWithUndefinedError()
}

func (u Undefined) Ge(any) (any, error) {
	return nil, u.failWithUndefinedError()
}

func (u Undefined) Pow(any) (any, error) {
	return nil, u.failWithUndefinedError()
}

func (u Undefined) RPow(any) (any, error) {
	return nil, u.failWithUndefinedError()
}

func (u Undefined) Pos() (any, error) {
	return nil, u.failWithUndefinedError()
}

func (u Undefined) Neg() (any, error) {
	return nil, u.failWithUndefinedError()
}

func (u Undefined) Bool() (bool, error) {
	u.logMessage()
	return false, nil
}

func (Undefined) Repr() string {
	return "Undefined"
}

func (u Undefined) String_() (string, error) {
	u.logMessage()
	return "", nil
}

func (u Undefined) Len() (int, error) {
	return 0, nil
}

func (u Undefined) Iter() ([]any, error) {
	return nil, nil
}

func (u Undefined) Call(...any) (any, error) {
	return nil, u.failWithUndefinedError()
}

func (u Undefined) GetItem(any) (any, error) {
	return nil, u.failWithUndefinedError()
}

func (u Undefined) Int(any) (int64, error) {
	return 0, u.failWithUndefinedError()
}

func (u Undefined) Float(any) (int64, error) {
	return 0, u.failWithUndefinedError()
}

func (u Undefined) Complex(any) (int64, error) {
	return 0, u.failWithUndefinedError()
}

func (u Undefined) Hash() (int64, error) {
	h, _ := strconv.ParseInt(fmt.Sprintf("%p", &u), 0, 64)
	return h, nil
}

func (su StrictUndefined) String_() (string, error) {
	su.logMessage()
	return "", su.failWithUndefinedError()
}

func (su StrictUndefined) Bool() (bool, error) {
	su.logMessage()
	return false, su.failWithUndefinedError()
}

func (su StrictUndefined) Eq() (any, error) {
	return nil, su.failWithUndefinedError()
}

func (su StrictUndefined) Ne() (any, error) {
	return nil, su.failWithUndefinedError()
}

func (su StrictUndefined) Hash() (int64, error) {
	return 0, su.failWithUndefinedError()
}

func (su StrictUndefined) Len() (int64, error) {
	return 0, su.failWithUndefinedError()
}

func (su StrictUndefined) Iter() ([]any, error) {
	return nil, su.failWithUndefinedError()
}

func (su StrictUndefined) Contains(any) (bool, error) {
	return false, su.failWithUndefinedError()
}

func objectTypeRepr(a any) string {
	return reflect.TypeOf(a).Name()
}

func (du DebugUndefined) String_() (string, error) {
	du.logMessage()
	name := "None"
	if du.name != nil {
		name = *du.name
	}

	var msg string
	if du.hint != nil {
		msg = fmt.Sprintf("undefined value printed: %s", *du.hint)
	} else if _, ok := du.obj.(utils.Missing); ok {
		msg = name
	} else {
		msg = fmt.Sprintf("no such element: '%s' [%s]", objectTypeRepr(du.obj), name)
	}

	return fmt.Sprintf("{{ %s }}", msg), nil
}

func (cu ChainableUndefined) HTML() (string, error) {
	return cu.String_()
}

func (cu ChainableUndefined) GetAttr() (any, error) {
	return cu, nil
}

func (cu ChainableUndefined) GetItem() (any, error) {
	return cu, nil
}
