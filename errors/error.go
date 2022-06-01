package errors

import "errors"

func New(text string) error {
	return errors.New(text)
}

func Wrap(classify, reason error) error {
	return errors.New(classify.Error() + " | " + reason.Error())
}
