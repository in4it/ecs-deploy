package service

func SetDeployDefaults(d *Deploy) {
	d.DeregistrationDelay = -1
	d.Stickiness.Duration = -1
}
