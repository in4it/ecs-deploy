package integrations

type Notification interface {
	LogFailure(msg string) error
	LogRecovery(msg string) error
}
