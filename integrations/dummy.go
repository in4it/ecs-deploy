package integrations

type Dummy struct{}

func NewDummy() *Dummy {
	return &Dummy{}
}

func (s *Dummy) LogFailure(message string) error {
	return nil
}

func (s *Dummy) LogRecovery(message string) error {
	return nil
}
