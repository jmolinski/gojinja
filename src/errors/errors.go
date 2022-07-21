package errors

import "fmt"

func TemplateNotFound(name string, msg string) error {
	// TODO reimplement jinja's logic.
	return fmt.Errorf("TEMPLATE NOT FOUND %s", name)
}
