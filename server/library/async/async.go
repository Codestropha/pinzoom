package async

type Starter interface {
	Start() <-chan struct{}
	StartAndWait() error
	Started() <-chan struct{}
	StartError() error
}

type Stopper interface {
	Stop() <-chan struct{}
	StopAndWait() error
	Stopped() <-chan struct{}
	StopError() error
}
