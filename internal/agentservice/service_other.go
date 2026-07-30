//go:build !windows

package agentservice

func IsService() (bool, error) {
	return false, nil
}

func Run(_ string, _ RunFunc) error {
	return ErrUnsupported
}

func Install(_, _, _, _ string) error {
	return ErrUnsupported
}

func Start(_ string) error {
	return ErrUnsupported
}

func Stop(_ string) error {
	return ErrUnsupported
}

func Query(_ string) (Status, error) {
	return Status{}, ErrUnsupported
}

func Uninstall(_ string) error {
	return ErrUnsupported
}
